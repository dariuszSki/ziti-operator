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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	zitiv1alpha1 "github.com/dariuszSki/ziti-operator/api/v1alpha1"
	ze "github.com/dariuszSki/ziti-operator/ziti-edge"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZitiRouterReconciler reconciles a ZitiRouter object
type ZitiRouterReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	recoder record.EventRecorder
}

// Ziti Router Config template

// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters/finalizers,verbs=update
// +kubebuilder:rbac:groups=ziti.dariuszski.dev,resources=zitirouters/events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

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
	if err := r.Get(ctx, req.NamespacedName, zitirouter); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Ziti Router resource not found. Ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Ziti Router")
		return ctrl.Result{}, err
	}

	// Initialize completion Resource status
	configMapReady := false
	statefulsetReady := false

	// Check if the Router ConfigMap already exists, if not create a new one
	foundCfgm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: zitirouter.Spec.RouterStatefulsetNamePrefix + "-config", Namespace: zitirouter.Namespace}, foundCfgm)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new Router ConfigMap
		cfgm := r.configMapForZitiRouter(zitirouter)
		log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cfgm.Namespace, "ConfigMap.Name", cfgm.Name)
		if err := r.Create(ctx, cfgm); err != nil {
			log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", cfgm.Namespace, "ConfigMap.Name", cfgm.Name)
			addStatusCondition(&zitirouter.Status, "RouterConfigurationNotReady", metav1.ConditionFalse, "RouterConfigurationNotReady", "Failed to add or update Ziti Router Configuration")
			return ctrl.Result{}, err
		}
		configMapReady = true
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	} else {
		// Ensure the ConfigMap is up-to-date
		cfgm := r.configMapForZitiRouter(zitirouter)
		// Compare relevant fields to determine if an update is needed
		if foundCfgm != cfgm {
			// ConfigMap has drifted, update it
			if err := r.Update(ctx, cfgm); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Configuration updated", "ConfigMap.Namespace", cfgm.Namespace, "ConfigMap.Name", cfgm.Name)
			r.recoder.Event(zitirouter, corev1.EventTypeNormal, "ConfigurationUpdated", "Router Configuration updated successfully")
		} else {
			log.Info("Configuration is up to date, no action required", "ConfigMap.Namespace", foundCfgm.Namespace, "ConfigMap.Name", foundCfgm.Name)
		}
	}

	// Check if the Statefulset already exists, if not create a new one
	foundSfs := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: zitirouter.Spec.RouterStatefulsetNamePrefix, Namespace: zitirouter.Namespace}, foundSfs)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new Statefulset
		sfs := r.statefulsetForZitiRouter(zitirouter)
		log.Info("Creating a new Statefulset", "Statefulset.Namespace", sfs.Namespace, "Statefulset.Name", sfs.Name)
		if err := r.Create(ctx, sfs); err != nil {
			log.Error(err, "Failed to create new Statefulset", "Statefulset.Namespace", sfs.Namespace, "Statefulset.Name", sfs.Name)
			addStatusCondition(&zitirouter.Status, "RouterStatefulsetNotReady", metav1.ConditionFalse, "RouterStatefulsetNotReady", "Failed to add or update Ziti Router Statefulset")
			return ctrl.Result{}, err
		}
		statefulsetReady = true
		// Requeue the request to ensure the Statefulset is created
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Stateful")
		return ctrl.Result{}, err
	}

	// Ensure the Statefulset replicas matches the desired state
	replicas := zitirouter.Spec.RouterReplicas
	if *foundSfs.Spec.Replicas != replicas {
		foundSfs.Spec.Replicas = &replicas
		if err := r.Update(ctx, foundSfs); err != nil {
			log.Error(err, "Failed to update Statefulset replicas", "Statefulset.Namespace", foundSfs.Namespace, "Statefulset.Name", foundSfs.Name)
			return ctrl.Result{}, err
		}
		// Requeue the request to ensure the correct state is achieved
		return ctrl.Result{Requeue: true}, nil
	}

	// Add or update Statefulset
	if configMapReady && statefulsetReady {
		addStatusCondition(&zitirouter.Status, "RouterConfigurationReady", metav1.ConditionTrue, "RouterConfigurationRead", "Ziti Router Configuration is ready")
		addStatusCondition(&zitirouter.Status, "RouterStatefulsetReady", metav1.ConditionTrue, "RouterStatefulsetReady", "Ziti Router Pods are ready")
	}

	log.Info("Reconciliation complete")
	// Update Ziti Router status to reflect that the Statefulset is available
	zitirouter.Status.Status.AvailableReplicas = foundSfs.Status.AvailableReplicas
	if err := r.Status().Update(ctx, zitirouter); err != nil {
		log.Error(err, "Failed to update Ziti Router status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func addStatusCondition(status *zitiv1alpha1.ZitiRouterStatus, condType string, statusType metav1.ConditionStatus, reason, message string) {
	for i, existingCondition := range status.Conditions {
		if existingCondition.Type == condType {
			// Condition already exists, update it
			status.Conditions[i].Status = statusType
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].LastTransitionTime = metav1.Now()
			return
		}
	}

	// Condition does not exist, add it
	condition := metav1.Condition{
		Type:               condType,
		Status:             statusType,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	status.Conditions = append(status.Conditions, condition)
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
			Replicas: &replicas,
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
								{
									Name: "SECRET_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: metav1.ObjectNameField,
										},
									},
								},
								{
									Name:  "ZITI_ENROLL_TOKEN",
									Value: zitirouter.Spec.ZitiRouterEnrollmentToken[0],
								},
								// {
								// 	Name: "ZITI_ENROLL_TOKEN",
								// 	ValueFrom: &corev1.EnvVarSource{
								// 		ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								// 			LocalObjectReference: corev1.LocalObjectReference{
								// 				Name: string(zitirouter.Spec.ZitiAdminEnrollmentToken[0]),
								// 			},
								// 		},
								// 	},
								// },
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

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiRouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recoder = mgr.GetEventRecorderFor("zitirouter-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiRouter{}). // Watch the primary resource
		Owns(&appsv1.StatefulSet{}).     // Watch the secondary resource (Statefulset)
		Owns(&corev1.ConfigMap{}).       // Watch the secondary resource (ConifgMap)
		Complete(r)
}
