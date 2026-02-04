/*
Copyright 2025.

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
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kaddyv1alpha1 "github.com/tmstff/kaddy/api/v1alpha1"
	"github.com/tmstff/kaddy/internal/util"
)

var log = logf.Log.WithName("kaddy_controller")

// KaddyReconciler reconciles a Kaddy object
type KaddyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *KaddyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kaddyv1alpha1.Kaddy{}).
		Named("kaddy_controller").
		Complete(r)
}

// +kubebuilder:rbac:groups=kaddy.quay.io,resources=kaddies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kaddy.quay.io,resources=kaddies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kaddy.quay.io,resources=kaddies/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *KaddyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	kaddy := &kaddyv1alpha1.Kaddy{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, kaddy)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// finalizer: https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/#handle-cleanup-on-deletion
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	err = r.reconcileConfigMap(ctx, kaddy)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcilePVC(ctx, kaddy)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileDeployment(ctx, kaddy)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, kaddy)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO handle route

	return ctrl.Result{Requeue: true}, nil
}

func (r *KaddyReconciler) reconcileConfigMap(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) error {
	configMapFromCluster := new(corev1.ConfigMap)
	configMapExists := true
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, configMapFromCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			configMapExists = false
		} else {
			return err
		}
	}

	if configMapExists {
		updatedConfigMap := r.configMapForKaddy(kaddy)
		if !reflect.DeepEqual(configMapFromCluster.Data, updatedConfigMap.Data) {
			if err := r.Update(ctx, updatedConfigMap); err != nil {
				return err
			}
		}
	} else {
		cm := r.configMapForKaddy(kaddy)
		err := r.Create(ctx, cm)
		if err != nil {
			return err
		}
		err = ctrl.SetControllerReference(kaddy, cm, r.Scheme) // for later automatic deletion
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *KaddyReconciler) configMapForKaddy(k *kaddyv1alpha1.Kaddy) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name,
			Namespace: k.Namespace,
		},
		Data: map[string]string{`Caddyfile`: r.caddyfileFor(k.Spec.LocalDomainNames)},
	}
}

func (*KaddyReconciler) caddyfileFor(localDomainNames []string) string {
	caddyfile := `{
      http_port 8080
      https_port 8443
    }`
	for _, dn := range localDomainNames {
		caddyfile += fmt.Sprintf(`
		%s {
		  respond "Hello from Kaddy!"
		  tls internal
		}`, dn)
	}
	return caddyfile
}

func (r *KaddyReconciler) reconcilePVC(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) error {
	pvcFromCluster := &corev1.PersistentVolumeClaim{}
	pvcExists := true
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, pvcFromCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			pvcExists = false
		} else {
			return err
		}
	}

	if pvcExists {
		updatedDeployment := r.pvcForKaddy(kaddy)
		if !reflect.DeepEqual(pvcFromCluster.Spec, updatedDeployment.Spec) {
			if err := r.Update(ctx, updatedDeployment); err != nil {
				return err
			}
		}
	} else {
		pvc := r.pvcForKaddy(kaddy)
		err := r.Create(ctx, pvc)
		if err != nil {
			return err
		}
		err = ctrl.SetControllerReference(kaddy, pvc, r.Scheme) // for later automatic deletion
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KaddyReconciler) pvcForKaddy(kaddy *kaddyv1alpha1.Kaddy) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kaddy.Name,
			Namespace: kaddy.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("500M"),
				},
			},
		},
	}

	return pvc
}

func (r *KaddyReconciler) reconcileDeployment(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) error {
	deploymentFromCluster := &appsv1.Deployment{}
	deploymentExists := true
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, deploymentFromCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			deploymentExists = false
		} else {
			return err
		}
	}

	if deploymentExists {
		updatedDeployment := r.deploymentForKaddy(ctx, kaddy)
		if !reflect.DeepEqual(deploymentFromCluster.Spec, updatedDeployment.Spec) {
			if err := r.Update(ctx, updatedDeployment); err != nil {
				return err
			}
		}
	} else {
		d := r.deploymentForKaddy(ctx, kaddy)
		err := r.Create(ctx, d)
		if err != nil {
			return err
		}
		err = ctrl.SetControllerReference(kaddy, d, r.Scheme) // for later automatic deletion
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KaddyReconciler) deploymentForKaddy(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) *appsv1.Deployment {
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kaddy.Name,
			Namespace: kaddy.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": kaddy.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": kaddy.Name},
					Annotations: map[string]string{"kaddy-config-map-checksum": r.computeConfigMapChecksum(ctx, kaddy)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "quay.io/hummingbird/caddy:latest",
						Name:  kaddy.Name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Name:          "http",
						}, {
							ContainerPort: 8443,
							Name:          "https",
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config-vol",
								MountPath: "/etc/caddy/Caddyfile",
								SubPath:   "Caddyfile",
							},
							{
								Name:      "data-vol",
								MountPath: "/data/caddy",
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "config-vol",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: kaddy.Name,
									},
								},
							},
						}, {
							Name: "data-vol",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: kaddy.Name + "-data",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *KaddyReconciler) computeConfigMapChecksum(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) string {
	configMapFromCluster := new(corev1.ConfigMap)
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, configMapFromCluster)
	if err != nil {
		log.Error(err, fmt.Sprintf("Could not get config map (name='%s', namespace='%s') to compute it's checksum", kaddy.Name, kaddy.Namespace))
		return ""
	}
	return util.CheckSumOf(configMapFromCluster.Data)
}

func (r *KaddyReconciler) reconcileService(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) error {
	serviceFromCluster := &corev1.Service{}
	serviceExists := true
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, serviceFromCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			serviceExists = false
		} else {
			return err
		}
	}

	if serviceExists {
		updatedService := r.serviceForKaddy(kaddy)
		if !reflect.DeepEqual(serviceFromCluster.Spec, updatedService.Spec) {
			if err := r.Update(ctx, updatedService); err != nil {
				return err
			}
		}
	} else {
		s := r.serviceForKaddy(kaddy)
		err := r.Create(ctx, s)
		if err != nil {
			return err
		}
		err = ctrl.SetControllerReference(kaddy, s, r.Scheme) // for later automatic deletion
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KaddyReconciler) serviceForKaddy(k *kaddyv1alpha1.Kaddy) *corev1.Service {
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name,
			Namespace: k.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "kaddy",
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       8443,
					TargetPort: intstr.FromInt(8443),
				},
			},
		},
	}
	return s
}
