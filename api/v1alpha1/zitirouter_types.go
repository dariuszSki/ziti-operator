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

// ZitiRouterSpec defines the desired state of ZitiRouter
type ZitiRouterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Controller Base URL https://${fqdn or ip}:port
	// +kubebuilder:validation:Optional
	ZitiMgmtApi string `json:"zitiMgmtApi"`

	// Controller Enrollment token to enroll admin user
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9_=]+)\.([a-zA-Z0-9_=]+)\.([a-zA-Z0-9_\-\+\/=]*)`
	// +kubebuilder:validation:Optional
	ZitiAdminEnrollmentToken string `json:"zitiAdminEnrollmentToken"`

	// Router Enrollment token to register it with Ziti Network
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9_=]+)\.([a-zA-Z0-9_=]+)\.([a-zA-Z0-9_\-\+\/=]*)`
	// +kubebuilder:validation:Optional
	ZitiRouterEnrollmentToken string `json:"zitiRouterEnrollmentToken"`

	// Router deployment name
	// +kubebuilder:validation:MaxLength:=63
	RouterDeploymentNamePrefix string `json:"routerDeploymentNamePrefix"`

	// Router Copies
	// +kubebuilder:default:=1
	RouterReplicas int32 `json:"routerReplicas"`

	// Router Containter Image Version
	// +kubebuilder:default:=latest
	ImageTag string `json:"imageTag"`

	// Router Log Level
	Debug string `json:"debug"`
}

// ZitiRouterStatus defines the observed state of ZitiRouter
type ZitiRouterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions of the ziti router custom resource
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ZitiRouter is the Schema for the zitirouters API
type ZitiRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiRouterSpec   `json:"spec,omitempty"`
	Status ZitiRouterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiRouterList contains a list of ZitiRouter
type ZitiRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiRouter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiRouter{}, &ZitiRouterList{})
}
