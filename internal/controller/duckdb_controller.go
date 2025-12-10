/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/nkhoshini/pondkeeper/api/v1alpha1"
)

const duckdbFinalizer = "cache.duckdb.org/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableDuckDB represents the status of the Pod reconciliation
	typeAvailableDuckDB = "Available"
	// typeDegradedDuckDB represents the status used when the custom resource is deleted and the finalizer operations are yet to occur.
	typeDegradedDuckDB = "Degraded"
)

// DuckDBReconciler reconciles a DuckDB object
type DuckDBReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cache.duckdb.org,resources=duckdbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.duckdb.org,resources=duckdbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.duckdb.org,resources=duckdbs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DuckDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the DuckDB instance
	duckdb := &cachev1alpha1.DuckDB{}
	err := r.Get(ctx, req.NamespacedName, duckdb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("duckdb resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get duckdb")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status is available
	if len(duckdb.Status.Conditions) == 0 {
		meta.SetStatusCondition(&duckdb.Status.Conditions, metav1.Condition{Type: typeAvailableDuckDB, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, duckdb); err != nil {
			log.Error(err, "Failed to update DuckDB status")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, duckdb); err != nil {
			log.Error(err, "Failed to re-fetch duckdb")
			return ctrl.Result{}, err
		}
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(duckdb, duckdbFinalizer) {
		log.Info("Adding Finalizer for DuckDB")
		if ok := controllerutil.AddFinalizer(duckdb, duckdbFinalizer); !ok {
			err = fmt.Errorf("finalizer for DuckDB was not added")
			log.Error(err, "Failed to add finalizer for DuckDB")
			return ctrl.Result{}, err
		}

		if err = r.Update(ctx, duckdb); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the DuckDB instance is marked to be deleted
	isDuckDBMarkedToBeDeleted := duckdb.GetDeletionTimestamp() != nil
	if isDuckDBMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(duckdb, duckdbFinalizer) {
			log.Info("Performing Finalizer Operations for DuckDB before delete CR")

			meta.SetStatusCondition(&duckdb.Status.Conditions, metav1.Condition{Type: typeDegradedDuckDB,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", duckdb.Name)})

			if err := r.Status().Update(ctx, duckdb); err != nil {
				log.Error(err, "Failed to update DuckDB status")
				return ctrl.Result{}, err
			}

			r.doFinalizerOperationsForDuckDB(duckdb)

			if err := r.Get(ctx, req.NamespacedName, duckdb); err != nil {
				log.Error(err, "Failed to re-fetch duckdb")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&duckdb.Status.Conditions, metav1.Condition{Type: typeDegradedDuckDB,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s were successfully accomplished", duckdb.Name)})

			if err := r.Status().Update(ctx, duckdb); err != nil {
				log.Error(err, "Failed to update DuckDB status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for DuckDB")
			if ok := controllerutil.RemoveFinalizer(duckdb, duckdbFinalizer); !ok {
				err = fmt.Errorf("finalizer for DuckDB was not removed")
				log.Error(err, "Failed to remove finalizer for DuckDB")
				return ctrl.Result{}, err
			}

			if err := r.Update(ctx, duckdb); err != nil {
				log.Error(err, "Failed to remove finalizer for DuckDB")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if the Pod already exists, if not create a new one
	found := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: duckdb.Name, Namespace: duckdb.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new Pod
		pod, err := r.podForDuckDB(duckdb)
		if err != nil {
			log.Error(err, "Failed to define new Pod resource for DuckDB")
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		if err = r.Create(ctx, pod); err != nil {
			log.Error(err, "Failed to create new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			return ctrl.Result{}, err
		}

		// Pod created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	// Update the DuckDB status with the pod names
	// List the pods for this duckdb's pod
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(duckdb.Namespace),
		client.MatchingLabels(labelsForDuckDB(duckdb.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "DuckDB.Namespace", duckdb.Namespace, "DuckDB.Name", duckdb.Name)
		return ctrl.Result{}, err
	}

	// Update status.Conditions if needed
	meta.SetStatusCondition(&duckdb.Status.Conditions, metav1.Condition{Type: typeAvailableDuckDB,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Pod for custom resource %s created successfully", duckdb.Name)})

	if err := r.Status().Update(ctx, duckdb); err != nil {
		log.Error(err, "Failed to update DuckDB status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// doFinalizerOperationsForDuckDB will perform the required operations before delete the CR.
func (r *DuckDBReconciler) doFinalizerOperationsForDuckDB(cr *cachev1alpha1.DuckDB) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted.

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))
}

// podForDuckDB returns a DuckDB Pod object
func (r *DuckDBReconciler) podForDuckDB(
	duckdb *cachev1alpha1.DuckDB) (*corev1.Pod, error) {
	// Use the image from Spec if provided, otherwise fallback or error
	// For now, we assume the user provides valid PodSpec or at least Image.
	// If Spec.Spec is empty, we construct a minimal one.
	image := duckdb.Spec.Image.Repository + ":" + duckdb.Spec.Image.Tag
	if duckdb.Spec.Image.Repository == "" {
		// Fallback to a default if not specified? Or error?
		// Example uses env var.
		var err error
		image, err = imageForDuckDB()
		if err != nil {
			// If no env var, and no spec, use a default placeholder or error
			// return nil, err
			// Let's use a placeholder for safety if no env var found
			image = "gizmodata/gizmosql:latest" // Placeholder
		}
	}

	if duckdb.Name == "" {
		// Generate a random kubernetes-friendly suffix for the name, e.g. 5-7 lowercase letters/numbers
		const nameSuffixLength = 6
		const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
		var suffix = make([]byte, nameSuffixLength)
		for i := range suffix {
			suffix[i] = letterBytes[time.Now().UnixNano()%int64(len(letterBytes))]
		}
		duckdb.Name = fmt.Sprintf("duckdb-%s", string(suffix))
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      duckdb.Name,
			Namespace: duckdb.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Image:           image,
				Name:            "duckdb",
				ImagePullPolicy: duckdb.Spec.Image.PullPolicy,
				Ports: []corev1.ContainerPort{{
					ContainerPort: duckdb.Spec.Port, // Use the port from spec
					Name:          "duckdb",
				}},
			}},
		},
	}

	// Set the ownerRef for the Pod
	if err := ctrl.SetControllerReference(duckdb, pod, r.Scheme); err != nil {
		return nil, err
	}
	return pod, nil
}

// labelsForDuckDB returns the labels for selecting the resources
func labelsForDuckDB(name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "duckdb",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": "DuckDBController",
	}
}

// imageForDuckDB gets the Operand image which is managed by this controller
// from the DUCKDB_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForDuckDB() (string, error) {
	var imageEnvVar = "DUCKDB_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("unable to find %s environment variable with the image", imageEnvVar)
	}
	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DuckDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.DuckDB{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
