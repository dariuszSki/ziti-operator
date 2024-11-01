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

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
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
	zitirouter := &zitiv1alpha1.ZitiRouter{}
	if err := r.Get(ctx, req.NamespacedName, zitirouter); err != nil {
		log.Error(err, "failed to get Ziti Router")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize completion Resource status
	configMapReady := false
	deploymentReady := false
	log.Info("Right before addOrUpdateRouterConfiguration")
	// Add or Update Router Configuration
	if err := r.addOrUpdateRouterConfiguration(ctx, zitirouter); err != nil {
		log.Error(err, "Failed to add or update Configuration Map for Ziti Router")
		addStatusCondition(&zitirouter.Status, "RouterConfigurationNotReady", metav1.ConditionFalse, "RouterConfigurationNotReady", "Failed to add or update Ziti Router Configuration")
		return ctrl.Result{}, err
	} else {
		configMapReady = true
	}

	// Add or update Deployment
	if err := r.addOrUpdateRouterDeployment(ctx, zitirouter); err != nil {
		log.Error(err, "Failed to add or update Deployment for Ziti Router")
		addStatusCondition(&zitirouter.Status, "RouterDeploymentNotReady", metav1.ConditionFalse, "RouterDeploymentNotReady", "Failed to add or update Ziti Router Deployment")
		return ctrl.Result{}, err
	} else {
		deploymentReady = true
	}

	if configMapReady && deploymentReady {
		addStatusCondition(&zitirouter.Status, "RouterConfigurationReady", metav1.ConditionTrue, "RouterConfigurationRead", "Ziti Router Configuration is ready")
		addStatusCondition(&zitirouter.Status, "RouterDeploymentReady", metav1.ConditionTrue, "RouterDeploymentReady", "Ziti Router Pods are ready")
	}
	log.Info("Reconciliation complete")
	if err := r.updateStatus(ctx, zitirouter); err != nil {
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

// Function to update the status of the zitirouter object
func (r *ZitiRouterReconciler) updateStatus(ctx context.Context, zitirouter *zitiv1alpha1.ZitiRouter) error {
	// Update the status of the Ziti Router object
	if err := r.Status().Update(ctx, zitirouter); err != nil {
		return err
	}

	return nil
}

func (r *ZitiRouterReconciler) addOrUpdateRouterConfiguration(ctx context.Context, zitirouter *zitiv1alpha1.ZitiRouter) error {
	log := log.FromContext(ctx)
	deployedRouterConfiguration := &corev1.ConfigMapList{}
	log.Info("In router addOrUpdateRouterConfiguration")
	err := r.List(ctx, deployedRouterConfiguration, &client.ListOptions{
		Namespace: zitirouter.ObjectMeta.Namespace,
	})
	if err != nil {
		return err
	}
	if len(deployedRouterConfiguration.Items) > 0 {
		// Search for router configmap if exists
		log.Info("list of configmaps", "List of Items", deployedRouterConfiguration.Items)
		for _, configmap := range deployedRouterConfiguration.Items {
			// If exists
			if configmap.ObjectMeta.Name == zitirouter.Spec.RouterDeploymentNamePrefix+"-config" {
				existingConfiguration := &configmap
				createNewConfiguration := createNewRouterConfiguration(zitirouter)
				// Compare relevant fields to determine if an update is needed
				if existingConfiguration != createNewConfiguration {
					// Deployment has drifted, update it
					existingConfiguration = createNewConfiguration
					if err := r.Update(ctx, existingConfiguration); err != nil {
						return err
					}
					log.Info("Configuration updated", "configuration", existingConfiguration.Name)
					r.recoder.Event(zitirouter, corev1.EventTypeNormal, "ConfigurationUpdated", "Router Configuration updated successfully")
				} else {
					log.Info("Configuration is up to date, no action required", "configuration", existingConfiguration.Name)
				}
				return nil
			}
		}

	}

	// Router Configuration does not exist, create it
	newConfiguration := createNewRouterConfiguration(zitirouter)
	if err := controllerutil.SetControllerReference(zitirouter, newConfiguration, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, newConfiguration); err != nil {
		return err
	}
	r.recoder.Event(zitirouter, corev1.EventTypeNormal, "ConfigurationCreated", "New Configuration created successfully")
	log.Info("New Configuration created", "name", zitirouter.ObjectMeta.Name)
	return nil
}

func (r *ZitiRouterReconciler) addOrUpdateRouterDeployment(ctx context.Context, zitirouter *zitiv1alpha1.ZitiRouter) error {
	log := log.FromContext(ctx)
	deployedRouterList := &appsv1.StatefulSetList{}
	labelSelector := labels.Set{"app": "zitirouter-" + zitirouter.ObjectMeta.Namespace}

	err := r.List(ctx, deployedRouterList, &client.ListOptions{
		Namespace:     zitirouter.ObjectMeta.Namespace,
		LabelSelector: labelSelector.AsSelector(),
	})
	if err != nil {
		return err
	}

	if len(deployedRouterList.Items) > 0 {
		// Deployment exists, update it
		log.Info("Found exisitng configuration", "configuration", deployedRouterList)
		existingDeployment := &deployedRouterList.Items[0] // Assuming only one deployment exists
		configuredDeployment := createRouterDeployment(zitirouter)

		// Compare relevant fields to determine if an update is needed
		if existingDeployment != configuredDeployment {
			// Deployment has drifted, update it
			existingDeployment.Spec = configuredDeployment.Spec
			if err := r.Update(ctx, existingDeployment); err != nil {
				return err
			}
			log.Info("Deployment updated", "deployment", existingDeployment.Name)
			r.recoder.Event(zitirouter, corev1.EventTypeNormal, "DeploymentUpdated", "Deployment updated successfully")
		} else {
			log.Info("Deployment is up to date, no action required", "deployment", existingDeployment.Name)
		}
		return nil
	}
	log.Info("Updating configuration", "configuration", deployedRouterList)
	// Deployment does not exist, create it
	newDeployment := createRouterDeployment(zitirouter)
	if err := controllerutil.SetControllerReference(zitirouter, newDeployment, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, newDeployment); err != nil {
		return err
	}
	r.recoder.Event(zitirouter, corev1.EventTypeNormal, "DeploymentCreated", "New Deployment created successfully")
	log.Info("New Deployment created", "name", zitirouter.ObjectMeta.Name)
	return nil
}

func createNewRouterConfiguration(zitirouter *zitiv1alpha1.ZitiRouter) *corev1.ConfigMap {
	log := log.Log
	c := &ze.Router{
		Version: 3,
		Identity: ze.Identity{
			Cert:           "/etc/ziti/config" + zitirouter.Spec.RouterDeploymentNamePrefix + ".cert",
			ServerCert:     "/etc/ziti/config" + zitirouter.Spec.RouterDeploymentNamePrefix + ".server.chain.cert",
			Key:            "/etc/ziti/config" + zitirouter.Spec.RouterDeploymentNamePrefix + ".key",
			Ca:             "/etc/ziti/config" + zitirouter.Spec.RouterDeploymentNamePrefix + ".cas",
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
					Advertise:         "tls::443",
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

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.RouterDeploymentNamePrefix + "-config",
			Namespace: zitirouter.ObjectMeta.Namespace,
			Labels: map[string]string{
				"app": "zitirouter-" + zitirouter.ObjectMeta.Namespace,
			},
		},
		Data: map[string]string{
			"ziti-router.yaml": string(routerConfig),
		},
	}
}

func createRouterDeployment(zitirouter *zitiv1alpha1.ZitiRouter) *appsv1.StatefulSet {
	replicas := zitirouter.Spec.RouterReplicas
	defaultConfigMode := int32(0444)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zitirouter.Spec.RouterDeploymentNamePrefix,
			Namespace: zitirouter.ObjectMeta.Namespace,
			Labels: map[string]string{
				"app": "zitirouter-" + zitirouter.ObjectMeta.Namespace,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "zitirouter-" + zitirouter.ObjectMeta.Namespace,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "zitirouter-" + zitirouter.ObjectMeta.Namespace,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "zitirouter",
							Image: "openziti/ziti-router:" + zitirouter.Spec.ImageTag,
							Env: []corev1.EnvVar{
								{
									Name:  "ZITI_ENROLL_TOKEN",
									Value: zitirouter.Spec.ZitiRouterEnrollmentToken,
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
									Value: "/etc/ziti",
								},
								{
									Name:  "ZITI_ROUTER_NAME",
									Value: zitirouter.Spec.RouterDeploymentNamePrefix,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
								},
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
									LocalObjectReference: corev1.LocalObjectReference{Name: zitirouter.Spec.RouterDeploymentNamePrefix + "-config"},
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
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiRouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.recoder = mgr.GetEventRecorderFor("zitirouter-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiRouter{}).
		Complete(r)
}
