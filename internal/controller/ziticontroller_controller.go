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

package controller

import (
	"context"
	"errors"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	zitiv1alpha1 "github.com/dariuszSki/ziti-operator/api/v1alpha1"
	ze "github.com/dariuszSki/ziti-operator/ziti-edge"
	"github.com/openziti/sdk-golang/ziti"
	corev1 "k8s.io/api/core/v1"
)

// ZitiControllerReconciler reconciles a ZitiController object
type ZitiControllerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=ziticontrollers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=ziticontrollers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=ziticontrollers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ZitiController object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ZitiControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var err error
	// Fetch Ziti Controller Instance
	ziticontroller := &zitiv1alpha1.ZitiController{}
	if err := r.Client.Get(ctx, req.NamespacedName, ziticontroller); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Ziti Controller Instance")
		return ctrl.Result{}, err
	}

	// Admin User Enrollment
	foundSecret := &corev1.Secret{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: ziticontroller.Spec.ControllerStatefulsetNamePrefix + "-secret", Namespace: ziticontroller.Namespace}, foundSecret)
	switch err != nil {
	case true:
		// Create
		if apierrors.IsNotFound(err) {
			//  If Admin User Token is not nill , then enroll
			if ziticontroller.Spec.ZitiAdminEnrollmentToken == "" {
				return ctrl.Result{}, errors.New("ZitiAdminEnrollmentToken is empty")
			}
			zitiCfg, err := ze.EnrollJwt(ziticontroller.Spec.ZitiAdminEnrollmentToken)
			if err != nil {
				return ctrl.Result{}, err
			}
			secret := r.secretForZitiController(ziticontroller, zitiCfg)
			if err := r.Client.Create(ctx, secret); err != nil {
				return ctrl.Result{}, err
			}
			// r.Recorder.Event(ziticontroller, corev1.EventTypeNormal, "ControllerSecretCreate", "Ziti Controller Secret is created")
			r.addControllerStatusCondition(&ziticontroller.Status, "ControllerSecretReady", metav1.ConditionTrue, "ControllerSecretReady", "Ziti Controller Secret is ready")
			return ctrl.Result{RequeueAfter: time.Minute}, nil

		} else {
			log.Error(err, "Failed to get Secret")
			return ctrl.Result{}, err
		}
	case false:
		// Update
		log.Info("Secret is up to date, no action required", "Secret.Namespace", foundSecret.Namespace, "Secret.Name", foundSecret.Name)

	}

	log.Info("Reconciliation complete")
	if err := r.Client.Status().Update(ctx, ziticontroller); err != nil {
		log.Error(err, "Failed to update Ziti Controller status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ZitiControllerReconciler) addControllerStatusCondition(status *zitiv1alpha1.ZitiControllerStatus, condType string, statusType metav1.ConditionStatus, reason string, message string) {
	for i, condition := range status.Conditions {
		if condition.Type == condType {
			// Condition already exists, update it
			status.Conditions[i].Status = statusType
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].LastTransitionTime = metav1.Now()
		} else {
			// Condition already exists, update it
			condition.Status = statusType
			condition.Reason = reason
			condition.Message = message
			condition.LastTransitionTime = metav1.Now()
			status.Conditions = append(status.Conditions, condition)
		}
	}
}

func (r *ZitiControllerReconciler) secretForZitiController(ziticontroller *zitiv1alpha1.ZitiController, zitiCfg *ziti.Config) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ziticontroller.Spec.ControllerStatefulsetNamePrefix + "-secret",
			Namespace: ziticontroller.ObjectMeta.Namespace,
			Labels: map[string]string{
				"app": "ziticontroller-" + ziticontroller.ObjectMeta.Namespace,
			},
		},
		StringData: map[string]string{
			"tls.key": zitiCfg.ID.Key,
			"tls.crt": zitiCfg.ID.Cert,
			"tls.ca":  zitiCfg.ID.CA,
		},
		Type: "kubernetes.io/tls",
	}
	// Set the ownerRef for Secret ensuring that it
	// will be deleted when Ziti Controller CR is deleted.
	controllerutil.SetControllerReference(ziticontroller, secret, r.Scheme)
	return secret
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiController{}).
		Owns(&corev1.Secret{}). // Watch the secondary resource (Secret)
		Complete(r)
}
