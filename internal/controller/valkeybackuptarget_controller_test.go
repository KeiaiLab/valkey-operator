/*
Copyright 2026 Keiailab.

ValkeyBackupTarget verifyCredentials + Reconcile 단위 테스트 — fake client.
envtest 의존 없음. ADR-0016.
*/

package controller

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// fakeTargetReconciler — fake client 에 target + (선택) Secret 1건 등록.
func fakeTargetReconciler(t *cachev1alpha1.ValkeyBackupTarget, sec *corev1.Secret) *ValkeyBackupTargetReconciler {
	scheme := runtime.NewScheme()
	_ = cachev1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	objs := []client.Object{t}
	if sec != nil {
		objs = append(objs, sec)
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&cachev1alpha1.ValkeyBackupTarget{}).
		Build()
	return &ValkeyBackupTargetReconciler{Client: c, Scheme: scheme}
}

func validS3Target(name, ns, secretName string) *cachev1alpha1.ValkeyBackupTarget {
	return &cachev1alpha1.ValkeyBackupTarget{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Generation: 1},
		Spec: cachev1alpha1.ValkeyBackupTargetSpec{
			Type: cachev1alpha1.BackupTargetTypeS3,
			S3: &cachev1alpha1.S3Spec{
				Endpoint: "https://s3.amazonaws.com",
				Region:   "ap-northeast-2",
				Bucket:   "valkey-backups",
				CredentialsSecretRef: cachev1alpha1.S3CredentialsSecretRef{
					Name: secretName,
				},
			},
		},
	}
}

func validCredsSecret(name, ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     []byte("AKIAEXAMPLE"),
			"AWS_SECRET_ACCESS_KEY": []byte("secret"),
		},
	}
}

// 1. type=GCS → UnsupportedType.
func TestBackupTarget_unsupportedType(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "creds")
	tgt.Spec.Type = cachev1alpha1.BackupTargetTypeGCS
	r := fakeTargetReconciler(tgt, nil)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "UnsupportedType" {
		t.Fatalf("expected UnsupportedType, got reason=%s ok=%v", reason, ok)
	}
}

// 2. type=S3 + S3 nil → MissingS3Spec.
func TestBackupTarget_missingS3Spec(t *testing.T) {
	tgt := &cachev1alpha1.ValkeyBackupTarget{
		ObjectMeta: metav1.ObjectMeta{Name: "vbt", Namespace: "ns", Generation: 1},
		Spec:       cachev1alpha1.ValkeyBackupTargetSpec{Type: cachev1alpha1.BackupTargetTypeS3},
	}
	r := fakeTargetReconciler(tgt, nil)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "MissingS3Spec" {
		t.Fatalf("expected MissingS3Spec, got reason=%s ok=%v", reason, ok)
	}
}

// 3. S3 fields incomplete → MissingFields.
func TestBackupTarget_missingFields(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "creds")
	tgt.Spec.S3.Bucket = "" // 누락
	r := fakeTargetReconciler(tgt, nil)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "MissingFields" {
		t.Fatalf("expected MissingFields, got reason=%s ok=%v", reason, ok)
	}
}

// 4. SecretRef.Name 미명시 → MissingSecretRef.
func TestBackupTarget_missingSecretRef(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "")
	r := fakeTargetReconciler(tgt, nil)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "MissingSecretRef" {
		t.Fatalf("expected MissingSecretRef, got reason=%s ok=%v", reason, ok)
	}
}

// 5. Secret not found → SecretNotFound.
func TestBackupTarget_secretNotFound(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "missing-secret")
	r := fakeTargetReconciler(tgt, nil)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "SecretNotFound" {
		t.Fatalf("expected SecretNotFound, got reason=%s ok=%v", reason, ok)
	}
}

// 6. Secret 의 access key 비어 있음 → MissingAccessKey.
func TestBackupTarget_missingAccessKey(t *testing.T) {
	sec := validCredsSecret("creds", "ns")
	delete(sec.Data, "AWS_ACCESS_KEY_ID")
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, sec)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "MissingAccessKey" {
		t.Fatalf("expected MissingAccessKey, got reason=%s ok=%v", reason, ok)
	}
}

// 7. Secret 의 secret key 비어 있음 → MissingSecretKey.
func TestBackupTarget_missingSecretKey(t *testing.T) {
	sec := validCredsSecret("creds", "ns")
	delete(sec.Data, "AWS_SECRET_ACCESS_KEY")
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, sec)

	reason, _, _, _, ok := r.verifyCredentials(context.Background(), tgt)
	if ok || reason != "MissingSecretKey" {
		t.Fatalf("expected MissingSecretKey, got reason=%s ok=%v", reason, ok)
	}
}

// 8. 모두 OK → CredentialsValid + ak/sk 추출.
func TestBackupTarget_credentialsValid(t *testing.T) {
	sec := validCredsSecret("creds", "ns")
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, sec)

	reason, msg, ak, sk, ok := r.verifyCredentials(context.Background(), tgt)
	if !ok || reason != "CredentialsValid" {
		t.Fatalf("expected CredentialsValid, got reason=%s ok=%v msg=%s", reason, ok, msg)
	}
	if ak != "AKIAEXAMPLE" || sk != "secret" {
		t.Fatalf("expected ak=AKIAEXAMPLE sk=secret, got ak=%s sk=%s", ak, sk)
	}
}

// fakeS3Client — verifyEndpoint 단위 테스트용 mock.
type fakeS3Client struct {
	exists  bool
	pingErr error
	host    string
}

func (f *fakeS3Client) Reachable(_ context.Context) (bool, error) { return f.exists, f.pingErr }
func (f *fakeS3Client) EndpointHost() string                      { return f.host }

func mockBuilder(client *fakeS3Client, buildErr error) s3ClientBuilder {
	return func(_ *cachev1alpha1.S3Spec, _, _ string) (s3Reachable, error) {
		if buildErr != nil {
			return nil, buildErr
		}
		return client, nil
	}
}

// 9. verifyEndpoint: BucketExists 가 true → EndpointReachable.
func TestBackupTarget_endpoint_reachable(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, nil)
	r.S3ClientBuilder = mockBuilder(&fakeS3Client{exists: true, host: "s3.fake"}, nil)

	reason, _, ok := r.verifyEndpoint(context.Background(), tgt, "ak", "sk")
	if !ok || reason != "EndpointReachable" {
		t.Fatalf("expected EndpointReachable, got reason=%s ok=%v", reason, ok)
	}
}

// 10. verifyEndpoint: BucketExists 가 false → BucketNotFound.
func TestBackupTarget_endpoint_bucketNotFound(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, nil)
	r.S3ClientBuilder = mockBuilder(&fakeS3Client{exists: false, host: "s3.fake"}, nil)

	reason, _, ok := r.verifyEndpoint(context.Background(), tgt, "ak", "sk")
	if ok || reason != "BucketNotFound" {
		t.Fatalf("expected BucketNotFound, got reason=%s ok=%v", reason, ok)
	}
}

// 11. verifyEndpoint: BucketExists 가 error → EndpointPingFailed.
func TestBackupTarget_endpoint_pingError(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, nil)
	r.S3ClientBuilder = mockBuilder(
		&fakeS3Client{pingErr: fmt.Errorf("network unreachable")},
		nil,
	)

	reason, _, ok := r.verifyEndpoint(context.Background(), tgt, "ak", "sk")
	if ok || reason != "EndpointPingFailed" {
		t.Fatalf("expected EndpointPingFailed, got reason=%s ok=%v", reason, ok)
	}
}

// 12. verifyEndpoint: client build 실패 → ClientBuildFailed.
func TestBackupTarget_endpoint_clientBuildFailed(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, nil)
	r.S3ClientBuilder = mockBuilder(nil, fmt.Errorf("invalid endpoint"))

	reason, _, ok := r.verifyEndpoint(context.Background(), tgt, "ak", "sk")
	if ok || reason != "ClientBuildFailed" {
		t.Fatalf("expected ClientBuildFailed, got reason=%s ok=%v", reason, ok)
	}
}

// Reconcile 통합 — Phase 전이 + LastVerifiedAt 기록.
//
// mock S3ClientBuilder 주입 → endpoint ping 도 통과 → Reachable.
func TestBackupTarget_reconcile_phaseTransition(t *testing.T) {
	sec := validCredsSecret("creds", "ns")
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, sec)
	r.S3ClientBuilder = mockBuilder(&fakeS3Client{exists: true, host: "s3.fake"}, nil)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "vbt", Namespace: "ns"}}

	// 1차 reconcile — 신규 → Pending → 즉시 검증 → endpoint ping → Reachable.
	res, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected requeue after Reachable, got %v", res)
	}

	got := &cachev1alpha1.ValkeyBackupTarget{}
	if err := r.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatalf("get target: %v", err)
	}
	if got.Status.Phase != cachev1alpha1.BackupTargetPhaseReachable {
		t.Fatalf("expected Phase=Reachable, got %s", got.Status.Phase)
	}
	if got.Status.LastVerifiedAt == nil {
		t.Fatalf("expected LastVerifiedAt non-nil")
	}
	// Ready=True + Reason=EndpointReachable.
	var ready *metav1.Condition
	for i := range got.Status.Conditions {
		if got.Status.Conditions[i].Type == "Ready" {
			ready = &got.Status.Conditions[i]
			break
		}
	}
	if ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready=True, got %+v", ready)
	}
	if ready.Reason != "EndpointReachable" {
		t.Fatalf("expected Reason=EndpointReachable, got %s", ready.Reason)
	}
}

// Reconcile — endpoint ping 실패 시 Phase=Unreachable + Reason=EndpointPingFailed.
func TestBackupTarget_reconcile_endpointFails(t *testing.T) {
	sec := validCredsSecret("creds", "ns")
	tgt := validS3Target("vbt", "ns", "creds")
	r := fakeTargetReconciler(tgt, sec)
	r.S3ClientBuilder = mockBuilder(
		&fakeS3Client{pingErr: fmt.Errorf("dial tcp: connection refused")},
		nil,
	)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "vbt", Namespace: "ns"}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := &cachev1alpha1.ValkeyBackupTarget{}
	if err := r.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatalf("get target: %v", err)
	}
	if got.Status.Phase != cachev1alpha1.BackupTargetPhaseUnreachable {
		t.Fatalf("expected Unreachable, got %s", got.Status.Phase)
	}
	var ready *metav1.Condition
	for i := range got.Status.Conditions {
		if got.Status.Conditions[i].Type == "Ready" {
			ready = &got.Status.Conditions[i]
			break
		}
	}
	if ready == nil || ready.Reason != "EndpointPingFailed" {
		t.Fatalf("expected Reason=EndpointPingFailed, got %+v", ready)
	}
}

// Reconcile 통합 — Secret 부재 시 Phase=Unreachable.
func TestBackupTarget_reconcile_unreachable(t *testing.T) {
	tgt := validS3Target("vbt", "ns", "missing")
	r := fakeTargetReconciler(tgt, nil) // secret 없음
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "vbt", Namespace: "ns"}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	got := &cachev1alpha1.ValkeyBackupTarget{}
	if err := r.Get(ctx, req.NamespacedName, got); err != nil {
		t.Fatalf("get target: %v", err)
	}
	if got.Status.Phase != cachev1alpha1.BackupTargetPhaseUnreachable {
		t.Fatalf("expected Phase=Unreachable, got %s", got.Status.Phase)
	}
}
