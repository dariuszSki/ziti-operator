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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	zitiv1alpha1 "github.com/dariuszSki/ziti-operator/api/v1alpha1"
	ze "github.com/dariuszSki/ziti-operator/ziti-edge"
	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_management_api_client/edge_router"
	rest_model_edge "github.com/openziti/edge-api/rest_model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZitiRouterReconciler reconciles a ZitiRouter object
type ZitiRouterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type JsonPatchEntry struct {
	OP    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ZitiRouter object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ZitiRouterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var err error
	// Fetch Ziti Router Instance
	zitirouter := &zitiv1alpha1.ZitiRouter{}
	if err := r.Client.Get(ctx, req.NamespacedName, zitirouter); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Ziti Router Instance")
		return ctrl.Result{}, err
	}

	// Fetch Ziti Controller Instance
	ziticontroller := &zitiv1alpha1.ZitiController{}
	if err := r.Get(ctx, req.NamespacedName, ziticontroller); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ZitiController Instance")
		return ctrl.Result{}, err
	}

	// Fetch Ziti Controller Instance Secret
	foundSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: "-secret", Namespace: zitirouter.Namespace}, foundSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Ziti Controller Secret")
		return ctrl.Result{}, err
	}

	// Router ConfigMap Reconcilation
	foundCfgm := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: zitirouter.Spec.RouterStatefulsetNamePrefix + "-config", Namespace: zitirouter.Namespace}, foundCfgm)
	switch err != nil {
	case true:
		// Create
		if apierrors.IsNotFound(err) {
			cfgm := r.configMapForZitiRouter(zitirouter)
			if err := r.Client.Create(ctx, cfgm); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(zitirouter, corev1.EventTypeNormal, "RouterConfigMapCreate", "Ziti Router Configuration is created")
			r.addRouterStatusCondition(&zitirouter.Status, "RouterConfigurationReady", metav1.ConditionTrue, "RouterConfigurationRead", "Ziti Router Configuration is ready")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else {
			log.Error(err, "Failed to get ConfigMap")
			return ctrl.Result{}, err
		}
	case false:
		// Update
		cfgm := r.configMapForZitiRouter(zitirouter)
		if foundCfgm.Data["ziti-router.yaml"] != cfgm.Data["ziti-router.yaml"] {

			if err := r.Client.Update(ctx, cfgm); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(zitirouter, corev1.EventTypeNormal, "ConfigurationUpdate", "Router Configuration is updated")
			r.addRouterStatusCondition(&zitirouter.Status, "RouterConfigurationReady", metav1.ConditionTrue, "RouterConfigurationRead", "Ziti Router Configuration is ready")
		} else {
			log.Info("Configuration is up to date, no action required", "ConfigMap.Namespace", foundCfgm.Namespace, "ConfigMap.Name", foundCfgm.Name)
		}
	}

	// Router Statefulset Reconcilation
	foundSfs := &appsv1.StatefulSet{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: zitirouter.Spec.RouterStatefulsetNamePrefix, Namespace: zitirouter.Namespace}, foundSfs)
	switch err != nil {
	case true:
		// Create
		if apierrors.IsNotFound(err) {
			sfs := r.statefulsetForZitiRouter(zitirouter)
			if err := r.Client.Create(ctx, sfs); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(zitirouter, corev1.EventTypeNormal, "RouterStatefulsetCreate", "Ziti Router Pods are being deployed")
			r.addRouterStatusCondition(&zitirouter.Status, "RouterStatefulsetReady", metav1.ConditionTrue, "RouterStatefulsetReady", "Ziti Router Pods are ready")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else {
			return ctrl.Result{}, err
		}
	case false:
		zitirouter.Spec.ZitiMgmtApi = "ziticontroller.Spec.ZitiMgmtApi"
		switch {
		// Delete Routers
		case *foundSfs.Spec.Replicas > *zitirouter.Spec.RouterReplicas:
			// Ziti Network
			zitiCfg, err := r.mgmtClientForZitiController(zitirouter, foundSecret)
			if err != nil {
				return ctrl.Result{}, err
			}
			for i := *zitirouter.Spec.RouterReplicas; i < *foundSfs.Spec.Replicas; i++ {
				_, err := updateZitiRouter(zitirouter.Spec.RouterStatefulsetNamePrefix+"-"+string(i), nil, zitiCfg)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
			// K8s
			foundSfs.Spec.Replicas = zitirouter.Spec.RouterReplicas
			if err := r.Client.Update(ctx, foundSfs); err != nil {
				log.Error(err, "Failed to update Statefulset replicas", "Statefulset.Namespace", foundSfs.Namespace, "Statefulset.Name", foundSfs.Name)
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(zitirouter, corev1.EventTypeNormal, "RouterStatefulsetDelete", "Statefulset Replicas were reduced to", &foundSfs.Spec.Replicas)
			r.addRouterStatusCondition(&zitirouter.Status, "RouterStatefulsetReady", metav1.ConditionTrue, "RouterStatefulsetReady", "Ziti Router Pods are ready")
			return ctrl.Result{Requeue: true}, nil
		// Add Routers
		case *foundSfs.Spec.Replicas < *zitirouter.Spec.RouterReplicas:
			// Ziti Network
			cost := int64(0)
			IsTunnelerEnabled := true
			isDisabled := false

			zitiCfg, err := r.mgmtClientForZitiController(zitirouter, foundSecret)
			if err != nil {
				return ctrl.Result{}, err
			}
			for i := *foundSfs.Spec.Replicas; i < *zitirouter.Spec.RouterReplicas; i++ {
				name := zitirouter.Spec.RouterStatefulsetNamePrefix + "-" + string(i)
				options := &rest_model_edge.EdgeRouterCreate{
					AppData:           nil,
					Cost:              &cost,
					Disabled:          &isDisabled,
					IsTunnelerEnabled: IsTunnelerEnabled,
					Name:              &name,
					RoleAttributes:    nil,
					Tags:              nil,
				}
				_, err := updateZitiRouter(name, options, zitiCfg)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
			// K8s
			foundSfs.Spec.Replicas = zitirouter.Spec.RouterReplicas
			if err := r.Update(ctx, foundSfs); err != nil {
				log.Error(err, "Failed to update Statefulset replicas", "Statefulset.Namespace", foundSfs.Namespace, "Statefulset.Name", foundSfs.Name)
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(zitirouter, corev1.EventTypeNormal, "RouterStatefulsetAdd", "Statefulset Replicas were increased to", &foundSfs.Spec.Replicas)
			r.addRouterStatusCondition(&zitirouter.Status, "RouterStatefulsetReady", metav1.ConditionTrue, "RouterStatefulsetReady", "Ziti Router Pods are ready")
			return ctrl.Result{Requeue: true}, nil
		default:
			// Router Enrollment Reconcilation
			podList := &corev1.PodList{}
			podListOpts := []client.ListOption{
				client.InNamespace(req.Namespace),
			}
			err = r.List(ctx, podList, podListOpts[0])
			if err != nil {
				return ctrl.Result{}, err
			}
			for _, pod := range podList.Items {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == "ContainersReady" {
						if condition.Status == "False" {
							// Create Router

							// Enroll Router
							valuePatch, _ := json.Marshal(zitirouter.Spec.ZitiRouterEnrollmentToken[pod.Name])
							jsonPatchData, _ := json.Marshal([]JsonPatchEntry{{OP: "replace", Path: "/data/" + zitirouter.Spec.RouterStatefulsetNamePrefix + "-token", Value: valuePatch}})
							if err := r.Client.Patch(ctx, foundCfgm, client.RawPatch(types.JSONPatchType, jsonPatchData)); err != nil {
								return ctrl.Result{}, err
							}
							r.Recorder.Eventf(zitirouter, corev1.EventTypeNormal, "RouterConfigMapUpdate", "Router Token Patched:", zitirouter.Spec.ZitiRouterEnrollmentToken[pod.Name])
							return ctrl.Result{RequeueAfter: time.Minute}, nil
						}
					}
				}
			}
		}
	}

	log.Info("Reconciliation complete")
	// Update Ziti Router status to reflect that the Statefulset is available
	zitirouter.Status.AvailableReplicas = foundSfs.Status.AvailableReplicas
	zitirouter.Status.ReadyReplicas = foundSfs.Status.ReadyReplicas
	if err := r.Client.Status().Update(ctx, zitirouter); err != nil {
		log.Error(err, "Failed to update Ziti Router status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ZitiRouterReconciler) addRouterStatusCondition(status *zitiv1alpha1.ZitiRouterStatus, condType string, statusType metav1.ConditionStatus, reason string, message string) {
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

func updateZitiRouter(name string, options *rest_model_edge.EdgeRouterCreate, zitiCfg *rest_management_api_client.ZitiEdgeManagement) (*edge_router.CreateEdgeRouterCreated, error) {
	var zId string = ""
	routerDetails, err := ze.GetEdgeRouterByName(name, zitiCfg)
	if err != nil {
		return nil, err
	}
	if options != nil && len(routerDetails.GetPayload().Data) == 0 {
		routerDetails, err := ze.CreateEdgeRouter(name, options, zitiCfg)
		if err != nil {
			return nil, err
		}
		return routerDetails, nil
	} else if len(routerDetails.GetPayload().Data) > 0 {
		for _, routerItem := range routerDetails.GetPayload().Data {
			zId = *routerItem.ID
		}
		err = ze.DeleteEdgeRouter(zId, zitiCfg)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil

}

func (r *ZitiRouterReconciler) configMapForZitiRouter(zitirouter *zitiv1alpha1.ZitiRouter) *corev1.ConfigMap {
	log := log.Log
	c := &ze.Router{
		Version: 3,
		Identity: ze.Identity{
			Cert:           "/etc/ziti/config/" + zitirouter.Spec.RouterStatefulsetNamePrefix + ".cert",
			ServerCert:     "/etc/ziti/config/" + zitirouter.Spec.RouterStatefulsetNamePrefix + ".server.chain.cert",
			Key:            "/etc/ziti/config/" + zitirouter.Spec.RouterStatefulsetNamePrefix + ".key",
			Ca:             "/etc/ziti/config/" + zitirouter.Spec.RouterStatefulsetNamePrefix + ".cas",
			AltServerCerts: ze.AltServerCerts{},
		},
		Controller: ze.Controller{
			Endpoint: "tls:" + zitirouter.Spec.ZitiMgmtApi,
		},
		Link: ze.Link{
			Dialers: []ze.LinkDialer{
				{
					Binding: "transport",
				},
			},
			Listeners: []ze.LinkListener{},
		},
		Listeners: []ze.EdgeListener{
			{
				Binding: "edge",
				Address: "tls:0.0.0.0:8443",
				Options: ze.EdgeListenerOptions{
					Advertise:         zitirouter.Spec.RouterStatefulsetNamePrefix + "." + zitirouter.ObjectMeta.Namespace + ".svc:443",
					ConnectTimeoutMs:  5000,
					GetSessionTimeout: 60,
				},
			},
			{
				Binding: "tunnel",
				Options: ze.EdgeListenerOptions{
					Mode:     "tproxy",
					Resolver: "udp://127.0.0.1:53",
					LanIf:    "lo",
				},
			},
		},
		CSR: ze.CSR{},
		Edge: ze.Edge{
			CSR: ze.CSR{
				Country:            "US",
				Province:           "NC",
				Locality:           "Charlotte",
				Organization:       "NetFoundry",
				OrganizationalUnit: "Ziti",
				Sans: ze.Sans{
					Dns: []string{
						"localhost",
					},
					Ip: []string{
						"127.0.0.1",
					},
				},
			},
		},
		Transport: ze.Transport{},
		Forwarder: ze.Forwarder{
			ListatencyProbeInterval: 0,
			XgressDialQueueLength:   1000,
			XgressDialWorkerCount:   128,
			LinkDialQueueLength:     1000,
			LinkDialWorkerCount:     32,
			RateLimitedQueueLength:  5000,
			RateLimitedWorkerCount:  64,
		},
	}
	routerConfig, err := c.MarshalYAML()
	if err != nil {
		log.Info("Error marshalling config", zitirouter.ObjectMeta.Name, err)
		return &corev1.ConfigMap{}
	}
	cfgm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.RouterStatefulsetNamePrefix + "-config",
			Namespace: zitirouter.ObjectMeta.Namespace,
			Labels: map[string]string{
				"app": "zitirouter-" + zitirouter.ObjectMeta.Namespace,
			},
		},
		Data: map[string]string{
			"ziti-router.yaml": string(routerConfig),
			zitirouter.Spec.RouterStatefulsetNamePrefix + "-token": "",
		},
	}
	// Set the ownerRef for ConfigMap ensuring that it
	// will be deleted when Ziti Router CR is deleted.
	controllerutil.SetControllerReference(zitirouter, cfgm, r.Scheme)
	return cfgm
}

// statefulsetForZitiRouter returns a Statefulset object for Ziti Router
func (r *ZitiRouterReconciler) statefulsetForZitiRouter(zitirouter *zitiv1alpha1.ZitiRouter) *appsv1.StatefulSet {
	replicas := zitirouter.Spec.RouterReplicas
	defaultConfigMode := int32(0444)
	rootUser := int64(0)

	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.RouterStatefulsetNamePrefix,
			Namespace: zitirouter.ObjectMeta.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": zitirouter.Spec.RouterStatefulsetNamePrefix},
			},
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.PersistentVolumeClaimRetentionPolicyType("Delete"),
				WhenScaled:  appsv1.PersistentVolumeClaimRetentionPolicyType("Delete"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": zitirouter.Spec.RouterStatefulsetNamePrefix},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "zitirouter",
							Image: "openziti/ziti-router:" + zitirouter.Spec.ImageTag,
							Env: []corev1.EnvVar{
								// {
								// 	Name: "SECRET_NAME",
								// 	ValueFrom: &corev1.EnvVarSource{
								// 		FieldRef: &corev1.ObjectFieldSelector{
								// 			FieldPath: metav1.ObjectNameField,
								// 		},
								// 	},
								{
									Name: "ZITI_ENROLL_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: zitirouter.Spec.RouterStatefulsetNamePrefix + "-config",
											},
											Key: zitirouter.Spec.RouterStatefulsetNamePrefix + "-token",
										},
									},
								},
								{
									Name:  "ZITI_BOOTSTRAP",
									Value: "true",
								},
								{
									Name:  "ZITI_BOOTSTRAP_ENROLLMENT",
									Value: "true",
								},
								{
									Name:  "ZITI_BOOTSTRAP_CONFIG",
									Value: "false",
								},
								{
									Name:  "ZITI_AUTO_RENEW_CERTS",
									Value: "true",
								},
								{
									Name:  "ZITI_HOME",
									Value: "/etc/ziti/config",
								},
								{
									Name:  "ZITI_ROUTER_NAME",
									Value: zitirouter.Spec.RouterStatefulsetNamePrefix,
								},
								{
									Name:  "DEBUG",
									Value: zitirouter.Spec.Debug,
								},
							},
							Command: []string{
								"/entrypoint.bash",
							},
							Args: []string{
								"run",
								"/etc/ziti/config/ziti-router.yaml",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN",
										"NET_BIND_SERVICE",
									},
									Drop: []corev1.Capability{
										"ALL",
									},
								},
								RunAsUser: &rootUser,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-data",
									MountPath: "/etc/ziti/config",
								},
								{
									Name:      "ziti-router-config",
									MountPath: "/etc/ziti/config/ziti-router.yaml",
									SubPath:   "ziti-router.yaml",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "ziti-router-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: zitirouter.Spec.RouterStatefulsetNamePrefix + "-config"},
									DefaultMode:          &defaultConfigMode,
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("50Mi"),
							},
						},
					},
				},
			},
		},
	}

	// Set the ownerRef for Statefulset ensuring that it
	// will be deleted when Ziti Router CR is deleted.
	controllerutil.SetControllerReference(zitirouter, sfs, r.Scheme)
	return sfs
}

// initiate Ziti Mgmt Client
func (r *ZitiRouterReconciler) mgmtClientForZitiController(zitirouter *zitiv1alpha1.ZitiRouter, secret *corev1.Secret) (*rest_management_api_client.ZitiEdgeManagement, error) {
	// Parse Ziti Admin Certs
	zitiTlsCertificate, _ := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
	parsedCert, err := x509.ParseCertificate(zitiTlsCertificate.Certificate[0])
	if err != nil {
		return nil, err
	}
	// Router Tokens Reconcilation
	zecfg := ze.Config{ApiEndpoint: zitirouter.Spec.ZitiMgmtApi, Cert: parsedCert, PrivateKey: zitiTlsCertificate.PrivateKey}
	zec, err := ze.Client(&zecfg)
	if err != nil {
		return nil, err
	}
	return zec, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiRouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiRouter{}). // Watch the primary resource
		Owns(&appsv1.StatefulSet{}).     // Watch the secondary resource (Statefulset)
		Owns(&corev1.ConfigMap{}).       // Watch the secondary resource (ConfigMap)
		Watches(&corev1.Pod{}, &handler.EnqueueRequestForObject{}).
		Watches(&corev1.Secret{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
