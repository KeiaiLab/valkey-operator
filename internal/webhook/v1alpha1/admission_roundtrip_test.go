/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Admission round-trip — webhook_suite_test 의 envtest 통합 검증. mongodb-
// operator it47 와 동일 패턴 (cross-cut UX 일관, ADR-0016).

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

var _ = Describe("Valkey webhook admission round-trip", func() {
	It("rejects Valkey with storage.size below 1Gi", func() {
		v := &cachev1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-smallstorage", Namespace: "default"},
			Spec: cachev1alpha1.ValkeySpec{
				Mode:     cachev1alpha1.ModeStandalone,
				Replicas: 1,
				Storage:  cachev1alpha1.StorageSpec{Size: resource.MustParse("512Mi")},
			},
		}
		err := k8sClient.Create(ctx, v)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("storage.size"))
	})

	It("rejects Valkey with TLS certManager omitempty trap", func() {
		v := &cachev1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-tlstrap", Namespace: "default"},
			Spec: cachev1alpha1.ValkeySpec{
				Mode:     cachev1alpha1.ModeStandalone,
				Replicas: 1,
				Storage:  cachev1alpha1.StorageSpec{Size: resource.MustParse("8Gi")},
				TLS: &cachev1alpha1.TLSSpec{
					Enabled: true,
					CertManager: &cachev1alpha1.CertManagerSpec{
						IssuerRef: cachev1alpha1.CertIssuerRef{Name: "", Kind: "ClusterIssuer"},
					},
				},
			},
		}
		err := k8sClient.Create(ctx, v)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		// hasCertMgr=false 로 평가 → 'requires either' 에러 확인.
		Expect(err.Error()).To(ContainSubstring("requires either"))
	})

	It("rejects Valkey with users[].passwordSecretRef trap", func() {
		v := &cachev1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-userstrap", Namespace: "default"},
			Spec: cachev1alpha1.ValkeySpec{
				Mode:     cachev1alpha1.ModeStandalone,
				Replicas: 1,
				Storage:  cachev1alpha1.StorageSpec{Size: resource.MustParse("8Gi")},
				Auth: cachev1alpha1.AuthSpec{
					Enabled: true,
					Users: []cachev1alpha1.ValkeyUser{{
						Name: "alice",
						// PasswordSecretRef.Name + Key 둘 다 비어있음.
						PasswordSecretRef: corev1.SecretKeySelector{},
					}},
				},
			},
		}
		err := k8sClient.Create(ctx, v)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("passwordSecretRef"))
	})

	// defect ④: 'autoFailover + replicasPerShard=0' reject invariant 는 제거됐다.
	// masters-only 토폴로지는 유효하며 명시 0 은 apiserver(CRD default=1 은 *부재* 시만
	// 적용) + mutating defaulter(0→1 clobber 제거) 양쪽에서 보존된다.

	It("rejects ValkeyCluster with totalNodes > 100", func() {
		vc := &cachev1alpha1.ValkeyCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-toomany", Namespace: "default"},
			Spec: cachev1alpha1.ValkeyClusterSpec{
				Shards:           50,
				ReplicasPerShard: ptr.To[int32](2), // total = 50 * 3 = 150
				Storage:          cachev1alpha1.StorageSpec{Size: resource.MustParse("8Gi")},
			},
		}
		err := k8sClient.Create(ctx, vc)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("must not exceed 100"))
	})

	It("accepts valid ValkeyCluster — admission round-trip 통과", func() {
		vc := &cachev1alpha1.ValkeyCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-clusterhappy", Namespace: "default"},
			Spec: cachev1alpha1.ValkeyClusterSpec{
				Shards:           3,
				ReplicasPerShard: ptr.To[int32](1),
				Storage:          cachev1alpha1.StorageSpec{Size: resource.MustParse("8Gi")},
			},
		}
		err := k8sClient.Create(ctx, vc)
		Expect(err).NotTo(HaveOccurred(), "valid cluster spec 은 admission 통과")
		Expect(k8sClient.Delete(ctx, vc)).To(Succeed())
	})

	It("accepts valid Valkey CR — admission round-trip 통과", func() {
		v := &cachev1alpha1.Valkey{
			ObjectMeta: metav1.ObjectMeta{Name: "rt-happy", Namespace: "default"},
			Spec: cachev1alpha1.ValkeySpec{
				Mode:     cachev1alpha1.ModeStandalone,
				Replicas: 1,
				Storage:  cachev1alpha1.StorageSpec{Size: resource.MustParse("8Gi")},
			},
		}
		err := k8sClient.Create(ctx, v)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, v)).To(Succeed())
	})
})
