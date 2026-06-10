/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

var _ = Describe("ValkeyCluster Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-cluster"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		valkeycluster := &cachev1alpha1.ValkeyCluster{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ValkeyCluster")
			err := k8sClient.Get(ctx, typeNamespacedName, valkeycluster)
			if err != nil && errors.IsNotFound(err) {
				resource := &cachev1alpha1.ValkeyCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: cachev1alpha1.ValkeyClusterSpec{
						Shards:           3,
						ReplicasPerShard: 1,
						Version:          cachev1alpha1.ValkeyVersion{Version: "8.1.6"},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &cachev1alpha1.ValkeyCluster{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ValkeyCluster")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ValkeyClusterReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When updating spec.version.version", func() {
		const resourceName = "test-valkeycluster-version-upgrade-20260507"

		ctx := context.Background()
		key := types.NamespacedName{Name: resourceName, Namespace: "default"}

		AfterEach(func() {
			resource := &cachev1alpha1.ValkeyCluster{}
			if err := k8sClient.Get(ctx, key, resource); err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				reconciler := &ValkeyClusterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("StatefulSet pod template image를 새 Valkey 버전으로 갱신한다", func() {
			reconciler := &ValkeyClusterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

			By("8.1.6 ValkeyCluster CR을 생성하고 STS image를 확인한다")
			resource := &cachev1alpha1.ValkeyCluster{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: cachev1alpha1.ValkeyClusterSpec{
					Shards:           3,
					ReplicasPerShard: 1,
					Version:          cachev1alpha1.ValkeyVersion{Version: "8.1.6"},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			stsKey := types.NamespacedName{Name: resources.StatefulSetName(resourceName), Namespace: "default"}
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, stsKey, sts)).To(Succeed())
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(cachev1alpha1.DefaultValkeyImage + ":8.1.6"))

			By("spec.version.version을 9.0.4로 변경하고 reconcile한다")
			current := &cachev1alpha1.ValkeyCluster{}
			Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
			current.Spec.Version.Version = "9.0.4"
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			By("STS pod template image가 9.0.4로 갱신된다")
			Expect(k8sClient.Get(ctx, stsKey, sts)).To(Succeed())
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(cachev1alpha1.DefaultValkeyImage + ":9.0.4"))
		})
	})

	Context("When reconciling Cluster modules", func() {
		const resourceName = "test-valkeycluster-modules-20260610"

		ctx := context.Background()
		key := types.NamespacedName{Name: resourceName, Namespace: "default"}

		AfterEach(func() {
			resource := &cachev1alpha1.ValkeyCluster{}
			if err := k8sClient.Get(ctx, key, resource); err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				reconciler := &ValkeyClusterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
				_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			}
		})

		It("spec.modules를 StatefulSet init-container와 loadmodule args로 전달한다", func() {
			reconciler := &ValkeyClusterReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

			resource := &cachev1alpha1.ValkeyCluster{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
				Spec: cachev1alpha1.ValkeyClusterSpec{
					Shards:           3,
					ReplicasPerShard: 1,
					Version:          cachev1alpha1.ValkeyVersion{Version: "9.0.4"},
					Modules: []cachev1alpha1.ModuleSpec{
						{Name: "valkey-search"},
						{Name: "valkey-json"},
						{Name: "valkey-bloom"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())

			stsKey := types.NamespacedName{Name: resources.StatefulSetName(resourceName), Namespace: "default"}
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, stsKey, sts)).To(Succeed())
			Expect(sts.Spec.Template.Spec.InitContainers).To(HaveLen(3))
			Expect(sts.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--loadmodule"))
			Expect(sts.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Name", resources.ModuleVolumeName)))
		})
	})
})
