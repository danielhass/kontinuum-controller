/*
Copyright 2021.

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

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ComponentOverlay struct {
	//+kubebuilder:validation:Required
	Type string `json:"type"`
	// https://github.com/fluxcd/helm-controller/blob/b467ea6cbbc7fef5e90df707a365eab3793b7b9c/api/v2beta1/helmrelease_types.go#L152
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
}

// OverlaySpec defines the desired state of Overlay
type OverlaySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Overlay. Edit overlay_types.go to remove/update
	Foo string `json:"foo,omitempty"`
	// Components of this overlay
	// +patchMergeKey=name
	// +patchStrategy=merge
	Components        []ComponentOverlay   `json:"components"`
	ManagedComponents []ComponentOverlay   `json:"managedComponents"`
	Selector          metav1.LabelSelector `json:"selector"`
}

func (cp OverlaySpec) ComponentsToMap() map[string]*apiextensionsv1.JSON {
	res := make(map[string]*apiextensionsv1.JSON)
	for _, v := range cp.Components {
		res[v.Type] = v.Values
	}
	return res
}

// OverlayStatus defines the observed state of Overlay
type OverlayStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Overlay is the Schema for the overlays API
type Overlay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OverlaySpec   `json:"spec,omitempty"`
	Status OverlayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OverlayList contains a list of Overlay
type OverlayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Overlay `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Overlay{}, &OverlayList{})
}
