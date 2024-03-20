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

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TempoMicroservicesSpec defines the desired state of TempoMicroservices
type TempoMicroservicesSpec struct {
	Chart  string               `json:"chart,omitempty"`
	Values apiextensionsv1.JSON `json:"values,omitempty"`
}

// TempoMicroservicesStatus defines the observed state of TempoMicroservices
type TempoMicroservicesStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TempoMicroservices is the Schema for the tempomicroservices API
type TempoMicroservices struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TempoMicroservicesSpec   `json:"spec,omitempty"`
	Status TempoMicroservicesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TempoMicroservicesList contains a list of TempoMicroservices
type TempoMicroservicesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TempoMicroservices `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TempoMicroservices{}, &TempoMicroservicesList{})
}
