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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kaddyv1alpha1 "github.com/tmstff/kaddy/api/v1alpha1"
	"github.com/tmstff/kaddy/internal/util"
)

// KaddyReconciler reconciles a Kaddy object
type KaddyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	// reconcile pvc

	err = r.reconcileDeployment(ctx, kaddy)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO handle service

	// TODO handle route

	return ctrl.Result{Requeue: true}, nil
}

func (r *KaddyReconciler) reconcileConfigMap(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) error {
	configMapFromCluster := new(corev1.ConfigMap)
	configMapExisted := true
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, configMapFromCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			configMapExisted = false
			cm := r.configMapForKaddy(kaddy)
			if err = r.Create(ctx, cm); err != nil {
				return err
			}
			err = ctrl.SetControllerReference(kaddy, cm, r.Scheme) // for later automatic deletion
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if configMapExisted {
		updatedConfigMap := r.configMapForKaddy(kaddy)
		if !reflect.DeepEqual(configMapFromCluster.Data, updatedConfigMap.Data) {
			if err := r.Update(ctx, updatedConfigMap); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *KaddyReconciler) reconcileDeployment(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) error {
	deploymentFromCluster := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, deploymentFromCluster)
	deploymentExisted := true
	if err != nil {
		if errors.IsNotFound(err) {
			deploymentExisted = false
			d, err := r.deploymentForKaddy(ctx, kaddy)
			if err != nil {
				return err
			}
			if err = r.Create(ctx, d); err != nil {
				return err
			}
			err = ctrl.SetControllerReference(kaddy, d, r.Scheme) // for later automatic deletion
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if deploymentExisted {
		updatedDeployment, err := r.deploymentForKaddy(ctx, kaddy)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(deploymentFromCluster.Spec, updatedDeployment.Spec) {
			if err := r.Update(ctx, updatedDeployment); err != nil {
				return err
			}
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

func (r *KaddyReconciler) deploymentForKaddy(ctx context.Context, k *kaddyv1alpha1.Kaddy) (*appsv1.Deployment, error) {
	replicas := int32(1)

	cmChecksum, err := r.computeConfigMapChecksum(ctx, k)
	if err != nil {
		return nil, err
	}

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name,
			Namespace: k.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": k.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": k.Name},
					Annotations: map[string]string{"kaddy-config-map-checksum": cmChecksum},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "quay.io/hummingbird/caddy:latest",
						Name:  k.Name,
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
										Name: k.Name,
									},
								},
							},
						}, {
							Name: "data-vol",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: k.Name + "-data",
								},
							},
						},
					},
				},
			},
		},
	}

	err = controllerutil.SetControllerReference(k, d, r.Scheme)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *KaddyReconciler) computeConfigMapChecksum(ctx context.Context, kaddy *kaddyv1alpha1.Kaddy) (string, error) {
	configMapFromCluster := new(corev1.ConfigMap)
	err := r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, configMapFromCluster)
	if err != nil {
		return "", err
	}
	return util.CheckSumOf(configMapFromCluster.Data), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KaddyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kaddyv1alpha1.Kaddy{}).
		Named("kaddy_controller").
		Complete(r)
}
