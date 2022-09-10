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
	//"kontinuum-controller.github.io/Kontinuum-controller/utils"
)

// OverlayReconciler reconciles a Overlay object
type OverlayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kontinuum-controller.kontinuum-controller.github.io,resources=overlays,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kontinuum-controller.kontinuum-controller.github.io,resources=overlays/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kontinuum-controller.kontinuum-controller.github.io,resources=overlays/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Overlay object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *OverlayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// name of our custom finalizer
	myFinalizerName := "kontinuum-controller.github.io/finalizer"

	log.Log.Info("Staring overlay reconlice for " + req.Name + " in " + req.Namespace)
	//pm := utils.NewMeasurement(utils.EVENT_GROUP_RECONCILE, utils.EVENT_OBJECT_OVERLAY, req.Name)

	// get targets based on label selector
	// TODO add finalizer logic
	var overlay kontinuumcontrollerv1alpha1.Overlay
	if err := r.Get(ctx, req.NamespacedName, &overlay); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// get targets based on label selector
	var targets kontinuumcontrollerv1alpha1.TargetList
	if err := r.List(ctx, &targets, client.InNamespace(req.Namespace), client.MatchingLabels(overlay.Spec.Selector.MatchLabels)); err != nil {
		log.Log.Error(err, "unable to list targets")
		return ctrl.Result{}, err
	}

	// handle finalizers
	if overlay.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted - add finalizers
		if !containsString(overlay.GetFinalizers(), myFinalizerName) {
			controllerutil.AddFinalizer(&overlay, myFinalizerName)
			if err := r.Update(ctx, &overlay); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(overlay.GetFinalizers(), myFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteOverlayFromTargets(ctx, &overlay, targets); err != nil {
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&overlay, myFinalizerName)
			if err := r.Update(ctx, &overlay); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		//pm.StopMeasurement()
		return ctrl.Result{}, nil
	}

	var targetOverlay kontinuumcontrollerv1alpha1.TargetOverlay
	targetOverlay.Name = req.Name
	targetOverlay.Components = overlay.Spec.Components

	var targetManagedOverlay kontinuumcontrollerv1alpha1.TargetOverlay
	targetManagedOverlay.Name = req.Name
	targetManagedOverlay.Components = overlay.Spec.ManagedComponents

	for _, target := range targets.Items {
		target.Spec.Overlays = appendOverlay(target.Spec.Overlays, targetOverlay)
		target.Spec.ManagedOverlays = appendOverlay(target.Spec.ManagedOverlays, targetManagedOverlay)
		if err := r.Update(ctx, &target); err != nil {
			log.Log.Error(err, "unable to update Target spec")
			return ctrl.Result{}, err
		}
	}

	log.Log.Info("Finished reconcile for " + req.Name + " in " + req.Namespace)
	//pm.StopMeasurement()
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OverlayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&kontinuumcontrollerv1alpha1.Overlay{}).
		// watch changes of targets to place reconcile requests if a label changes
		Watches(
			&source.Kind{Type: &kontinuumcontrollerv1alpha1.Target{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForTarget),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Complete(r)
}

func appendStringUnique(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			return slice
		}
	}
	return append(slice, s)
}

func appendOverlay(base []kontinuumcontrollerv1alpha1.TargetOverlay, item kontinuumcontrollerv1alpha1.TargetOverlay) []kontinuumcontrollerv1alpha1.TargetOverlay {

	for i, w := range base {
		if w.Name == item.Name {
			base[i].Components = item.Components
			return base
		}
	}

	return append(base, item)
}

func (r *OverlayReconciler) findObjectsForTarget(target client.Object) []reconcile.Request {
	matchingOverlays := &kontinuumcontrollerv1alpha1.OverlayList{}
	listOps := &client.ListOptions{
		Namespace: target.GetNamespace(),
	}
	err := r.List(context.TODO(), matchingOverlays, listOps)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, item := range matchingOverlays.Items {
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

func (r *OverlayReconciler) deleteOverlayFromTargets(ctx context.Context, overlay *kontinuumcontrollerv1alpha1.Overlay, targets kontinuumcontrollerv1alpha1.TargetList) error {
	log.Log.Info("Removing overlay " + overlay.Name + " from targets")
	for _, target := range targets.Items {
		for i, targetOverlay := range target.Spec.Overlays {
			if targetOverlay.Name == overlay.Name {
				target.Spec.Overlays[i] = target.Spec.Overlays[len(target.Spec.Overlays)-1]
				target.Spec.Overlays = target.Spec.Overlays[:len(target.Spec.Overlays)-1]
			}
		}
		for i, targetManagedOverlay := range target.Spec.ManagedOverlays {
			if targetManagedOverlay.Name == overlay.Name {
				target.Spec.ManagedOverlays[i] = target.Spec.ManagedOverlays[len(target.Spec.ManagedOverlays)-1]
				target.Spec.ManagedOverlays = target.Spec.ManagedOverlays[:len(target.Spec.ManagedOverlays)-1]
			}
		}
		if err := r.Update(ctx, &target); err != nil {
			log.Log.Error(err, "unable to update Target spec")
			return err
		}
	}
	return nil
}
