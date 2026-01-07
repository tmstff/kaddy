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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kaddy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *KaddyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	kaddy := &kaddyv1alpha1.Kaddy{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, kaddy)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO update ConfigMap
	configMap := new(corev1.ConfigMap)
	configMapExisted := true
	err = r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define and create a new deployment.
			cm := r.configMapForKaddy(kaddy)
			if err = r.Create(ctx, cm); err != nil {
				return ctrl.Result{}, err
			}
			configMapExisted = false
		} else {
			return ctrl.Result{}, err
		}
	}

	updated := false
	if configMapExisted {
		updated, err = r.updateConfigMap(ctx, configMap, kaddy)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	restartPods := !configMapExisted || updated

	// handle pvc

	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: kaddy.Name, Namespace: kaddy.Namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			// Define and create a new deployment.
			dep, err := r.deploymentForKaddy(kaddy)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err = r.Create(ctx, dep); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	// TODO only update if necessary
	deployment, err = r.deploymentForKaddy(kaddy)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Update(ctx, deployment); err != nil {
		return ctrl.Result{}, err
	}

	// restart pods if necessary
	print(restartPods)

	// TODO handle service

	// TODO handle route

	return ctrl.Result{Requeue: true}, nil
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

func (r *KaddyReconciler) updateConfigMap(ctx context.Context, configMap *corev1.ConfigMap, kaddy *kaddyv1alpha1.Kaddy) (updated bool, err error) {
	updateRequired := false
	localDomainNames := kaddy.Spec.LocalDomainNames
	caddyfile := r.caddyfileFor(localDomainNames)
	if configMap.Data[`Caddyfile`] != caddyfile {
		configMap.Data[`Caddyfile`] = caddyfile
		updateRequired = true
		if err := r.Update(ctx, configMap); err != nil {
			return false, err
		}
	}
	return updateRequired, nil
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

func (r *KaddyReconciler) deploymentForKaddy(k *kaddyv1alpha1.Kaddy) (*appsv1.Deployment, error) {
	replicas := int32(1)
	label := labelForApp(k.Name)

	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name,
			Namespace: k.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: label,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: label,
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

	err := controllerutil.SetControllerReference(k, d, r.Scheme)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func labelForApp(name string) map[string]string {
	return map[string]string{"app": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *KaddyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kaddyv1alpha1.Kaddy{}).
		Named("kaddy_controller").
		Complete(r)
}
