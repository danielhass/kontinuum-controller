/*
Copyright 2022 The Kontinuum Controller Authors.

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

// S3 bucket config
type S3Config struct {
	CredentialSecretName string `json:"credentialSecretName,omitempty"`
	BucketName           string `json:"bucketName,omitempty"`
	Folder               string `json:"folder"`
	Region               string `json:"region,omitempty"`
	// +kubebuilder:default:=AWS
	Provider string `json:"provider,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

type TargetWorkload struct {
	Name       string      `json:"name,omitempty"`
	Components []Component `json:"components,omitempty"`
}

type TargetManagedWorkload struct {
	Name              string             `json:"name,omitempty"`
	ManagedComponents []ManagedComponent `json:"managedComponents,omitempty"`
}

type TargetOverlay struct {
	Name       string             `json:"name,omitempty"`
	Components []ComponentOverlay `json:"components,omitempty"`
}

func (to TargetOverlay) ComponentsToMap() map[string]*apiextensionsv1.JSON {
	res := make(map[string]*apiextensionsv1.JSON)
	for _, v := range to.Components {
		res[v.Type] = v.Values
	}
	return res
}

// TargetSpec defines the desired state of Target
type TargetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Target. Edit target_types.go to remove/update
	Foo string `json:"foo,omitempty"`
	// paused targets do not upload deployment manifests to remote storage
	// +kubebuilder:default=false
	Paused           bool                    `json:"paused,omitempty"`
	S3               S3Config                `json:"s3,omitempty"`
	Workloads        []TargetWorkload        `json:"workloads,omitempty"`
	ManagedWorkloads []TargetManagedWorkload `json:"managedWorkloads,omitempty"`
	Overlays         []TargetOverlay         `json:"overlays,omitempty"`
	ManagedOverlays  []TargetOverlay         `json:"managedOverlays,omitempty"`
}

// TargetStatus defines the observed state of Target
type TargetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Workloads           []TargetWorkload   `json:"workloads,omitempty"`
	ManagedWorkloads    []Component        `json:"managedWorkloads,omitempty"`
	LastUploadTimestamp metav1.Time        `json:"lastUploadTimestamp,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Target is the Schema for the targets API
type Target struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TargetSpec   `json:"spec,omitempty"`
	Status TargetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TargetList contains a list of Target
type TargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Target `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Target{}, &TargetList{})
}
