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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kaddyv1alpha1 "github.com/tmstff/kaddy/api/v1alpha1"
)

var _ = Describe("Kaddy Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-kaddy"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		kaddy := &kaddyv1alpha1.Kaddy{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Kaddy")
			err := k8sClient.Get(ctx, typeNamespacedName, kaddy)
			if err != nil && errors.IsNotFound(err) {
				resource := &kaddyv1alpha1.Kaddy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      typeNamespacedName.Name,
						Namespace: typeNamespacedName.Namespace,
					},
					Spec: kaddyv1alpha1.KaddySpec{
						LocalDomainNames: []string{"test-domain"},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup all resources created")
			kaddy := &kaddyv1alpha1.Kaddy{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, kaddy)).To(Succeed())
			Expect(k8sClient.Delete(ctx, kaddy)).To(Succeed())

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, cm)).To(Succeed())
			Expect(k8sClient.Delete(ctx, cm)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pvc)).To(Succeed())
			Expect(k8sClient.Delete(ctx, pvc)).To(Succeed())

			d := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, d)).To(Succeed())
			Expect(k8sClient.Delete(ctx, d)).To(Succeed())

			s := &corev1.Service{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, s)).To(Succeed())
			Expect(k8sClient.Delete(ctx, s)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &KaddyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Tests for statically configured values are omitted on purpose

			cm := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, typeNamespacedName, cm)
			Expect(err).NotTo(HaveOccurred())
			Expect(cm.Data["Caddyfile"]).NotTo(BeNil())
			// check if content sounds plausible taking the domain name from the CR into account
			Expect(cm.Data["Caddyfile"]).To(ContainSubstring("test-domain {"))

			pvc := &corev1.PersistentVolumeClaim{}
			err = k8sClient.Get(ctx, typeNamespacedName, pvc)
			Expect(err).NotTo(HaveOccurred())

			d := &appsv1.Deployment{}
			err = k8sClient.Get(ctx, typeNamespacedName, d)
			Expect(err).NotTo(HaveOccurred())

			s := &corev1.Service{}
			err = k8sClient.Get(ctx, typeNamespacedName, s)
			Expect(err).NotTo(HaveOccurred())
		})
		// TODO test if updating of existing resources works properly
		// TODO test deletion?
	})
})
