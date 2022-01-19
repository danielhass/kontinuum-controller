/*
Copyright 2022.

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

package controllers

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kontinuumcontrollerv1alpha1 "kontinuum-controller.github.io/Kontinuum-controller/api/v1alpha1"
	"kontinuum-controller.github.io/Kontinuum-controller/utils"
)

// WorkloadReconciler reconciles a Workload object
type WorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kontinuum-controller.kontinuum-controller.github.io,resources=workloads,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kontinuum-controller.kontinuum-controller.github.io,resources=workloads/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kontinuum-controller.kontinuum-controller.github.io,resources=workloads/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workload object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *WorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info(time.Now().Format(time.RFC3339Nano) + "," + "reconlice,start," + req.Name)
	pm := utils.NewMeasurement(utils.EVENT_GROUP_RECONCILE, utils.EVENT_OBJECT_WORKLOAD, req.Name)

	// name of our custom finalizer
	myFinalizerName := "kontinuum-controller.github.io/finalizer"

	// log.Log.Info(fmt.Sprintf("%s", req.NamespacedName))
	log.Log.Info("Starting reconcile for " + req.Name + " in " + req.Namespace)

	// get changed workload from k8s
	var workload kontinuumcontrollerv1alpha1.Workload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// get targets based on label selector
	var targets kontinuumcontrollerv1alpha1.TargetList
	if err := r.List(ctx, &targets, client.InNamespace(req.Namespace), client.MatchingLabels(workload.Spec.Selector.MatchLabels)); err != nil {
		log.Log.Error(err, "unable to list targets")
		return ctrl.Result{}, err
	}

	// handle finalizers
	if workload.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted - add finalizers
		if !containsString(workload.GetFinalizers(), myFinalizerName) {
			controllerutil.AddFinalizer(&workload, myFinalizerName)
			if err := r.Update(ctx, &workload); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(workload.GetFinalizers(), myFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteWorkloadsFromTargets(ctx, &workload, targets); err != nil {
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&workload, myFinalizerName)
			if err := r.Update(ctx, &workload); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		pm.StopMeasurement()
		return ctrl.Result{}, nil
	}

	// update targets with new workload
	var targetWorkloadComp kontinuumcontrollerv1alpha1.TargetWorkload
	targetWorkloadComp.Name = req.Name
	targetWorkloadComp.Components = workload.Spec.Components

	var targetManagedWorkloadComp kontinuumcontrollerv1alpha1.TargetManagedWorkload
	targetManagedWorkloadComp.Name = req.Name
	targetManagedWorkloadComp.ManagedComponents = workload.Spec.ManagedComponents

	for _, target := range targets.Items {
		//target.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
		target.Spec.Workloads = appendWorkload(target.Spec.Workloads, targetWorkloadComp)
		target.Spec.ManagedWorkloads = appendManagedWorkload(target.Spec.ManagedWorkloads, targetManagedWorkloadComp)

		if err := r.Update(ctx, &target); err != nil {
			log.Log.Error(err, "unable to update Target status")
			return ctrl.Result{}, err
		}
	}

	log.Log.Info("Finished reconcile for " + req.Name + " in " + req.Namespace)
	pm.StopMeasurement()
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kontinuumcontrollerv1alpha1.Workload{}).
		// watch changes of targets to place reconcile requests if a label changes
		Watches(
			&source.Kind{Type: &kontinuumcontrollerv1alpha1.Target{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForTarget),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Complete(r)
}

func (r *WorkloadReconciler) deleteWorkloadsFromTargets(ctx context.Context, workload *kontinuumcontrollerv1alpha1.Workload, targets kontinuumcontrollerv1alpha1.TargetList) error {
	log.Log.Info("Removing workload " + workload.Name + " from targets")
	for _, target := range targets.Items {
		for i, targetWorkload := range target.Spec.Workloads {
			if targetWorkload.Name == workload.Name {
				target.Spec.Workloads[i] = target.Spec.Workloads[len(target.Spec.Workloads)-1]
				target.Spec.Workloads = target.Spec.Workloads[:len(target.Spec.Workloads)-1]
			}
		}
		for i, targetManagedWorkload := range target.Spec.ManagedWorkloads {
			if targetManagedWorkload.Name == workload.Name {
				target.Spec.ManagedWorkloads[i] = target.Spec.ManagedWorkloads[len(target.Spec.ManagedWorkloads)-1]
				target.Spec.ManagedWorkloads = target.Spec.ManagedWorkloads[:len(target.Spec.ManagedWorkloads)-1]
			}
		}
		if err := r.Update(ctx, &target); err != nil {
			log.Log.Error(err, "unable to update Target status")
			return err
		}
	}
	return nil
}

func appendWorkload(base []kontinuumcontrollerv1alpha1.TargetWorkload, item kontinuumcontrollerv1alpha1.TargetWorkload) []kontinuumcontrollerv1alpha1.TargetWorkload {

	for i, w := range base {
		if w.Name == item.Name {
			base[i].Components = item.Components
			return base
		}
	}

	return append(base, item)
}

func appendManagedWorkload(base []kontinuumcontrollerv1alpha1.TargetManagedWorkload, item kontinuumcontrollerv1alpha1.TargetManagedWorkload) []kontinuumcontrollerv1alpha1.TargetManagedWorkload {

	for i, w := range base {
		if w.Name == item.Name {
			base[i].ManagedComponents = item.ManagedComponents
			return base
		}
	}

	return append(base, item)
}

// originally from: https://book.kubebuilder.io/reference/using-finalizers.html
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *WorkloadReconciler) findObjectsForTarget(target client.Object) []reconcile.Request {
	matchingWorkloads := &kontinuumcontrollerv1alpha1.WorkloadList{}
	listOps := &client.ListOptions{
		Namespace: target.GetNamespace(),
	}
	err := r.List(context.TODO(), matchingWorkloads, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, item := range matchingWorkloads.Items {
		selector, _ := metav1.LabelSelectorAsSelector(&item.Spec.Selector)
		// check if workload matches watched target
		if selector.Matches(labels.Set(target.GetLabels())) {
			//log.Log.Info("Found matching label! " + item.Name + " > " + target.GetName())
			rrequest := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				},
			}
			// only reconcile targets that match
			requests = append(requests, rrequest)
		}
	}
	return requests
}
