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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TempoMicroservicesSpec defines the desired state of TempoMicroservices
type TempoMicroservicesSpec struct {
	Chart  string               `json:"chart,omitempty"`
	Values apiextensionsv1.JSON `json:"values,omitempty"`
}

// PodStatusMap defines the type for mapping pod status to pod name.
type PodStatusMap map[corev1.PodPhase][]string

// ComponentStatus defines the status of each component.
type ComponentStatus struct {
	// Compactor is a map of the pod status of the Compactor pods.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Compactor",order=1,xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	Compactor PodStatusMap `json:"compactor"`

	// Distributor is a map of the pod status of the Distributor pods.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Distributor",order=2,xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	Distributor PodStatusMap `json:"distributor"`

	// Ingester is a map of the pod status of the Ingester pods.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Ingester",order=3,xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	Ingester PodStatusMap `json:"ingester"`

	// Querier is a map of the pod status of the Querier pods.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Querier",order=4,xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	Querier PodStatusMap `json:"querier"`

	// QueryFrontend is a map of the pod status of the QueryFrontend pods.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="QueryFrontend",order=5,xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	QueryFrontend PodStatusMap `json:"queryFrontend"`

	// Observatorium is a map of the pod status of the Observatorium pods.
	//
	// +optional
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Observatorium",order=6,xDescriptors="urn:alm:descriptor:com.tectonic.ui:podStatuses"
	Observatorium PodStatusMap `json:"observatorium"`
}

// ConditionStatus defines the status of a condition (e.g. ready, failed, pending or configuration error).
type ConditionStatus string

const (
	// ConditionReady defines that all components are ready.
	ConditionReady ConditionStatus = "Ready"
	// ConditionFailed defines that one or more components are in a failed state.
	ConditionFailed ConditionStatus = "Failed"
	// ConditionPending defines that one or more components are in a pending state.
	ConditionPending ConditionStatus = "Pending"
	// ConditionConfigurationError defines that there is a configuration error.
	ConditionConfigurationError ConditionStatus = "ConfigurationError"
)

// ConditionReason defines possible reasons for each condition.
type ConditionReason string

const (
	// ReasonReady defines a healthy tempo instance.
	ReasonReady ConditionReason = "Ready"
	// ReasonFailedComponents when all/some Tempo components fail to roll out.
	ReasonFailedComponents ConditionReason = "FailedComponents"
	// ReasonPendingComponents when all/some Tempo components pending dependencies.
	ReasonPendingComponents ConditionReason = "PendingComponents"
	// ReasonFailedReconciliation when the operator failed to reconcile.
	ReasonFailedReconciliation ConditionReason = "FailedReconciliation"
	// ReasonInvalidStorageConfig defines that the object storage configuration is invalid (missing or incomplete storage secret).
	ReasonInvalidStorageConfig ConditionReason = "InvalidStorageConfig"
)

// TempoMicroservicesStatus defines the observed state of TempoMicroservices
type TempoMicroservicesStatus struct {
	// Components provides summary of all Tempo pod status, grouped per component.
	//
	// +kubebuilder:validation:Optional
	Components ComponentStatus `json:"components,omitempty"`

	// Conditions of the Tempo deployment health.
	//
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
