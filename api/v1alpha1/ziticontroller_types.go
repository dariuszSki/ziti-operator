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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ZitiControllerSpec defines the desired state of ZitiController
type ZitiControllerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Controller FQDN:PORT
	ZitiMgmtApi string `json:"zitiMgmtApi,omitempty"`

	// Controller Enrollment token to enroll admin user
	//+kubebuilder:validation:Pattern:=`^([a-zA-Z0-9_=]+)\.([a-zA-Z0-9_=]+)\.([a-zA-Z0-9_\-\+\/=]*)`
	ZitiAdminEnrollmentToken string `json:"zitiAdminEnrollmentToken,omitempty"`

	// Controller deployment name
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:default:=ziti-controller
	ControllerStatefulsetNamePrefix string `json:"controllerStatefulsetNamePrefix"`
}

// ZitiControllerStatus defines the observed state of ZitiController
type ZitiControllerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions of the ziti router custom resource
	Conditions []*metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ZitiController is the Schema for the ziticontrollers API
type ZitiController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiControllerSpec   `json:"spec,omitempty"`
	Status ZitiControllerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiControllerList contains a list of ZitiController
type ZitiControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiController{}, &ZitiControllerList{})
}
