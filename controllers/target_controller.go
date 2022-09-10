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
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	goyaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"

	crdv1alpha1 "kontinuum-controller.github.io/Kontinuum-controller/api/v1alpha1"
)

// TargetReconciler reconciles a Target object
type TargetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=crd.kontinuum-controller.github.io,resources=targets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.kontinuum-controller.github.io,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.kontinuum-controller.github.io,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Target object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("Staring target reconcile for " + req.Name + " in " + req.Namespace)

	// get targets based on label selector
	var target crdv1alpha1.Target
	if err := r.Get(ctx, req.NamespacedName, &target); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if target is paused
	if target.Spec.Paused {
		// return as we do not reconcile paused targets
		log.Log.Info("Target " + req.Name + " paused, skipping reconcile")
		return ctrl.Result{}, nil
	}

	// TODO currently doesn't work - the object has been modified loop
	// conditionReadyFound := false
	// // build new condition
	// readyCondReconcile := metav1.Condition{
	// 	LastTransitionTime: metav1.Now(),
	// 	Message:            "Reconciliation in progress",
	// 	Reason:             "ReconciliationRunning",
	// 	Status:             metav1.ConditionFalse,
	// 	Type:               "Ready",
	// }
	// for i, condition := range target.Status.Conditions {
	// 	if condition.Type == "Ready" {
	// 		conditionReadyFound = true
	// 		target.Status.Conditions[i] = readyCondReconcile
	// 	}
	// }
	// if !conditionReadyFound {
	// 	target.Status.Conditions = append(target.Status.Conditions, readyCondReconcile)
	// }
	// if err := r.Status().Update(ctx, &target); err != nil {
	// 	log.Log.Error(err, "unable to update Target status")
	// 	return ctrl.Result{}, err
	// }

	dir, err := ioutil.TempDir("tmp", "continuum-controller")
	if err != nil {
		log.Log.Error(err, "unable to create tmp dir")
	}
	defer os.RemoveAll(dir)

	// merge all managed workload specs
	managedComponentsMap := make(map[string]apiextensionsv1.JSON)
	for _, managedWorkload := range target.Spec.ManagedWorkloads {
		for _, mc := range managedWorkload.ManagedComponents {
			if _, ok := managedComponentsMap[mc.Type]; !ok {
				managedComponentsMap[mc.Type] = apiextensionsv1.JSON{}
			}
		}
	}

	var managedComponents []crdv1alpha1.Component

	// TODO - this for loop needs some refactoring as it duplicates the workload loop below a lot
	for managedComponentType, managedComponentValues := range managedComponentsMap {

		for _, overlay := range target.Spec.ManagedOverlays {
			overlayMap := overlay.ComponentsToMap()
			// check if overlay for specific component type exists
			// if not we do not need to overlay anything
			if overlayValues, exists := overlayMap[managedComponentType]; exists {
				// overlay exists
				mergedValues, err := mergeJson(&managedComponentValues, overlayValues)
				if err != nil {
					log.Log.Error(err, "error during workload value conversion")
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}

				//managedComponentsMap[managedComponentType] = mergedValues
				managedComponentValues = mergedValues
			}
		}

		genComponent := crdv1alpha1.Component{
			Name:   managedComponentType,
			Type:   managedComponentType,
			Values: &managedComponentValues,
		}

		managedComponents = append(managedComponents, genComponent)

		// write Helm release to file
		releaseFilename := "managed-" + managedComponentType + ".yaml"

		// build HelmRelease for specific component
		log.Log.Info("Generating Helm Release for managed component" + managedComponentType)
		var helmRelease = generateHelmRelease("managed", genComponent)

		err := writeHelmReleaseFile(dir, releaseFilename, helmRelease)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	for _, workload := range target.Spec.Workloads {
		for i := 0; i < len(workload.Components); i++ {
			for _, overlay := range target.Spec.Overlays {
				overlayMap := overlay.ComponentsToMap()
				// check if overlay for specific component type exists
				// if not we do not need to overlay anything
				if overlayValues, exists := overlayMap[workload.Components[i].Type]; exists {
					// overlay exists
					mergedValues, err := mergeJson(workload.Components[i].Values, overlayValues)
					if err != nil {
						log.Log.Error(err, "error during workload value conversion")
						return ctrl.Result{}, client.IgnoreNotFound(err)
					}

					workload.Components[i].Values = &mergedValues
				}
			}

			// write Helm release to file
			releaseFilename := workload.Name + "-" + workload.Components[i].Name + ".yaml"

			// build HelmRelease for specific component
			log.Log.Info("Generating Helm Release for " + workload.Name + "/" + workload.Components[i].Name)
			var helmRelease = generateHelmRelease(workload.Name, workload.Components[i])

			err := writeHelmReleaseFile(dir, releaseFilename, helmRelease)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// generate .keep file
	err = os.WriteFile(dir+"/.keep", nil, 0644)
	if err != nil {
		log.Log.Error(err, "unable to write .keep file")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// get S3 secret for current target
	var s3Secret v1.Secret
	var s3SecretName types.NamespacedName
	s3SecretName.Name = target.Spec.S3.CredentialSecretName
	s3SecretName.Namespace = req.Namespace
	if err := r.Get(ctx, s3SecretName, &s3Secret); err != nil {
		log.Log.Error(err, "unable to fetch S3 secret")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// sync files to S3
	log.Log.Info("Uploading Helm Releases for " + target.Name + " to S3 " + target.Spec.S3.BucketName)
	cmd := exec.Command(
		"rclone",
		"sync",
		"--s3-provider", target.Spec.S3.Provider,
		"--s3-access-key-id", string(s3Secret.Data["access-key-id"]),
		"--s3-secret-access-key", string(s3Secret.Data["secret-access-key"]),
		"--s3-region", target.Spec.S3.Region,
		"--s3-endpoint", target.Spec.S3.Endpoint,
		"--checksum",
		dir,
		":s3:"+target.Spec.S3.BucketName+"/"+target.Spec.S3.Folder+"/"+target.Name)
	//cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Log.Error(err, "error during S3 sync")
	}

	// update conditions
	// readyCondReconcile = metav1.Condition{
	// 	LastTransitionTime: metav1.Now(),
	// 	Message:            "Upload to bucket " + target.Spec.S3.BucketName + " successfull",
	// 	Reason:             "ReconciliationSucceeded",
	// 	Status:             metav1.ConditionTrue,
	// 	Type:               "Ready",
	// }
	// for i, condition := range target.Status.Conditions {
	// 	if condition.Type == "Ready" {
	// 		target.Status.Conditions[i] = readyCondReconcile
	// 	}
	// }

	target.Status.Workloads = target.Spec.Workloads
	target.Status.ManagedWorkloads = managedComponents
	target.Status.LastUploadTimestamp = metav1.Now()
	if err := r.Status().Update(ctx, &target); err != nil {
		log.Log.Error(err, "unable to update Target status")
		return ctrl.Result{}, err
	}

	log.Log.Info("Finished target reconlice for " + req.Name + " in " + req.Namespace)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TargetReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&crdv1alpha1.Target{}).
		// ref: https://stuartleeks.com/posts/kubebuilder-event-filters-part-2-update/
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(ue event.UpdateEvent) bool {
				oldGen := ue.ObjectOld.GetGeneration()
				newGen := ue.ObjectNew.GetGeneration()
				return oldGen != newGen
			},
		}).
		WithOptions(
			controller.Options{MaxConcurrentReconciles: 1},
		).
		Complete(r)
}

// write a flux HelmRelease into a file
func writeHelmReleaseFile(dir string, filename string, helmRelease fluxhelmv2beta1.HelmRelease) error {
	mHelmRelease, err := yaml.Marshal(helmRelease)
	if err != nil {
		return err
	}

	err = os.WriteFile(dir+"/"+filename, mHelmRelease, 0644)
	if err != nil {
		log.Log.Error(err, "unable to write HelmRelease file")
		return err
	}
	return nil
}

func generateHelmRelease(nameingPrefix string, component crdv1alpha1.Component) fluxhelmv2beta1.HelmRelease {
	var helmRelease fluxhelmv2beta1.HelmRelease
	helmRelease.TypeMeta.APIVersion = "helm.toolkit.fluxcd.io/v2beta1"
	helmRelease.TypeMeta.Kind = "HelmRelease"
	helmRelease.Name = nameingPrefix + "-" + component.Name
	helmRelease.Namespace = "flux-system"
	helmRelease.Spec.TargetNamespace = "default"
	helmRelease.Spec.Values = component.Values
	helmRelease.Spec.ReleaseName = nameingPrefix + "-" + component.Name
	helmRelease.Spec.Chart = fluxhelmv2beta1.HelmChartTemplate{
		Spec: fluxhelmv2beta1.HelmChartTemplateSpec{
			Chart:     component.Type,
			SourceRef: fluxhelmv2beta1.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "kontinuum-catalog", Namespace: "kontinuum-system"},
		},
	}
	return helmRelease
}

func mergeJson(base *apiextensionsv1.JSON, overlay *apiextensionsv1.JSON) (apiextensionsv1.JSON, error) {
	var result []byte

	// just return overlay if base is nil
	if base.Size() == 0 {
		return *overlay, nil
	}

	mapBase, err := jsonToMap(base)
	if err != nil {
		log.Log.Error(err, "error during workload value conversion")
		return apiextensionsv1.JSON{Raw: result}, client.IgnoreNotFound(err)
	}
	mapOverlay, err := jsonToMap(overlay)
	if err != nil {
		log.Log.Error(err, "error during overlay value conversion")
		return apiextensionsv1.JSON{Raw: result}, client.IgnoreNotFound(err)
	}

	res := mergeYaml(mapBase, mapOverlay)
	mResult, err := goyaml.Marshal(&res)

	result, err = yaml.YAMLToJSON(mResult)

	return apiextensionsv1.JSON{Raw: result}, nil
}

func jsonToMap(json *apiextensionsv1.JSON) (map[interface{}]interface{}, error) {
	result := make(map[interface{}]interface{})
	mJson, err := yaml.Marshal(json)
	if err != nil {
		return result, err
	}
	goyaml.Unmarshal(mJson, &result)
	return result, nil
}

func mergeYaml(orig map[interface{}]interface{}, overlay map[interface{}]interface{}) map[interface{}]interface{} {
	for key, elem := range overlay {
		// log.Println(key, " - ", elem)
		// log.Println("Key type: ", reflect.TypeOf(elem).Kind())

		rt := reflect.TypeOf(elem)
		switch rt.Kind() {
		case reflect.Slice:
			// Slices probably need to be replaced as a whole
			// log.Println("Found Slice")
			orig[key] = elem
		case reflect.Map:
			// Found another map/subdocument, go deeper in yaml structure
			// log.Println("Found Map")
			// check if key exists in original
			if orig[key] == nil {
				// if not just copy value from overlay
				orig[key] = elem
			} else {
				// otherwise deep merge further
				origMap := orig[key].(map[interface{}]interface{})
				overlayMap := elem.(map[interface{}]interface{})
				mergeYaml(origMap, overlayMap)
			}
		default:
			orig[key] = elem
		}
	}

	return orig
}
