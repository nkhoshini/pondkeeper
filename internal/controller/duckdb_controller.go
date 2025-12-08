/*
Copyright 2024.

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	databasev1alpha1 "github.com/nimakhoshini/pondkeeper/api/v1alpha1"
)

// DuckDBReconciler reconciles a DuckDB object
type DuckDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=database.pondkeeper.io,resources=duckdbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.pondkeeper.io,resources=duckdbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.pondkeeper.io,resources=duckdbs/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DuckDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Fetch the DuckDB instance
	duckDB := &databasev1alpha1.DuckDB{}
	err := r.Get(ctx, req.NamespacedName, duckDB)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Define the desired Deployment object
	deployment := r.desiredDeployment(duckDB)

	// Check if the Deployment already exists
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		l.Info("Creating a new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.Create(ctx, deployment)
		if err != nil {
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	replicas := duckDB.Spec.Replicas
	if replicas == nil {
		defaultReplicas := int32(1)
		replicas = &defaultReplicas
	}

	if *found.Spec.Replicas != *replicas {
		found.Spec.Replicas = replicas
		err = r.Update(ctx, found)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the DuckDB status with the pod names
	// List the pods for this duckdb's deployment
	// podList := &corev1.PodList{}
	// listOpts := []client.ListOption{
	// 	client.InNamespace(duckDB.Namespace),
	// 	client.MatchingLabels(labelsForDuckDB(duckDB.Name)),
	// }
	// if err = r.List(ctx, podList, listOpts...); err != nil {
	// 	l.Error(err, "Failed to list pods", "DuckDB.Namespace", duckDB.Namespace, "DuckDB.Name", duckDB.Name)
	// 	return ctrl.Result{}, err
	// }

	// Update status.AvailableReplicas
	if duckDB.Status.AvailableReplicas != found.Status.AvailableReplicas {
		duckDB.Status.AvailableReplicas = found.Status.AvailableReplicas
		err := r.Status().Update(ctx, duckDB)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DuckDBReconciler) desiredDeployment(duckDB *databasev1alpha1.DuckDB) *appsv1.Deployment {
	replicas := duckDB.Spec.Replicas
	if replicas == nil {
		defaultReplicas := int32(1)
		replicas = &defaultReplicas
	}

	image := duckDB.Spec.Image
	if image == "" {
		image = "gizmodata/gizmosql:latest"
	}

	labels := labelsForDuckDB(duckDB.Name)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      duckDB.Name,
			Namespace: duckDB.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Name:  "duckdb",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080, // Default for gizmosql?
							Name:          "http",
						}},
					}},
				},
			},
		},
	}
	// Set DuckDB instance as the owner and controller
	ctrl.SetControllerReference(duckDB, dep, r.Scheme)
	return dep
}

func labelsForDuckDB(name string) map[string]string {
	return map[string]string{"app": "duckdb", "duckdb_cr": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DuckDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1alpha1.DuckDB{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
