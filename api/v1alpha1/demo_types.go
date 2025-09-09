package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DemoPhase string

const (
	DemoPhasePending     DemoPhase = "Pending"
	DemoPhaseRunning     DemoPhase = "Running"
	DemoPhaseDegraded    DemoPhase = "Degraded"
	DemoPhaseFailed      DemoPhase = "Failed"
	DemoPhaseTerminating DemoPhase = "Terminating"
)

// DemoSpec defines the desired state of Demo
type DemoSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html
	Name *string `json:"name,omitempty"`

	NodePort *int32 `json:"nodePort,omitempty"`
}

// DemoStatus defines the observed state of Demo.
type DemoStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase      DemoPhase          `json:"phase,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Demo is the Schema for the demoes API
type Demo struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Demo
	// +required
	Spec DemoSpec `json:"spec"`

	// status defines the observed state of Demo
	// +optional
	Status DemoStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// DemoList contains a list of Demo
type DemoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Demo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Demo{}, &DemoList{})
}
