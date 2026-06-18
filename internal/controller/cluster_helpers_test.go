/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 순수함수 단위테스트 — Reconcile 통합 path 와 무관하게 알고리즘 회귀를 차단한다.
// 외부 인프라 (envtest / Valkey 컨테이너) 불필요.
package controller

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
	vk "github.com/keiailab/valkey-operator/internal/valkey"
)

func testCtx() context.Context { return context.Background() }

// scalePolicyTestReconciler — fake client 에 STS 가 *currentReplicas* 로 존재 (nil → 미존재).
func scalePolicyTestReconciler(currentReplicas *int32) *ValkeyClusterReconciler {
	scheme := runtime.NewScheme()
	_ = cachev1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	objects := []client.Object{}
	if currentReplicas != nil {
		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resources.StatefulSetName("vk"),
				Namespace: "ns",
			},
			Spec: appsv1.StatefulSetSpec{Replicas: currentReplicas},
		}
		objects = append(objects, sts)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	return &ValkeyClusterReconciler{Client: c, Scheme: scheme}
}

func TestPodAddresses_ordering(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1

	got := podAddresses(vc)
	want := []string{
		"vk-0.vk-headless.ns.svc:6379",
		"vk-1.vk-headless.ns.svc:6379",
		"vk-2.vk-headless.ns.svc:6379",
		"vk-3.vk-headless.ns.svc:6379",
		"vk-4.vk-headless.ns.svc:6379",
		"vk-5.vk-headless.ns.svc:6379",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("podAddresses ordering mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestPodAddresses_zeroNodes(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 0
	if got := podAddresses(vc); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

// CreateCluster 의 round-robin 규칙: replicas[i] → primaries[i%shards].
// buildShardStatus 가 같은 매핑을 역으로 표현하는지 검증.
func TestBuildShardStatus_3x1(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1

	got := buildShardStatus(vc)
	if len(got) != 3 {
		t.Fatalf("want 3 shards, got %d", len(got))
	}

	// shard 0: primary vk-0, replica vk-3 (replica i=0 → primary 0%3=0)
	// shard 1: primary vk-1, replica vk-4 (i=1 → 1%3=1)
	// shard 2: primary vk-2, replica vk-5 (i=2 → 2%3=2)
	expectedPrimaries := []string{"vk-0", "vk-1", "vk-2"}
	expectedReplicas := [][]string{{"vk-3"}, {"vk-4"}, {"vk-5"}}
	for i, sh := range got {
		if sh.PrimaryPod != expectedPrimaries[i] {
			t.Errorf("shard %d primary: got %s want %s", i, sh.PrimaryPod, expectedPrimaries[i])
		}
		if !reflect.DeepEqual(sh.ReplicaPods, expectedReplicas[i]) {
			t.Errorf("shard %d replicas: got %v want %v", i, sh.ReplicaPods, expectedReplicas[i])
		}
	}

	// slot 분배: 16384 / 3 = 5461 (마지막 shard 가 +1 흡수해 5462).
	wantSlots := []int32{5461, 5461, 5462}
	wantRanges := []string{"0-5460", "5461-10921", "10922-16383"}
	for i, sh := range got {
		if sh.AssignedSlots != wantSlots[i] {
			t.Errorf("shard %d slots: got %d want %d", i, sh.AssignedSlots, wantSlots[i])
		}
		if sh.SlotRange != wantRanges[i] {
			t.Errorf("shard %d range: got %s want %s", i, sh.SlotRange, wantRanges[i])
		}
	}

	// 합계 검증 — 16384 정확히.
	var total int32
	for _, sh := range got {
		total += sh.AssignedSlots
	}
	if total != 16384 {
		t.Fatalf("slot total: got %d want 16384", total)
	}
}

// 3 shards × 2 replicas — 9 pod, replica 인덱스 i=0..5
// CreateCluster 매핑: replica i → primary (i%3). pod ordinal = shards + i = 3+i.
//
//	shard 0: replicas at i=0 (pod 3), i=3 (pod 6)
//	shard 1: replicas at i=1 (pod 4), i=4 (pod 7)
//	shard 2: replicas at i=2 (pod 5), i=5 (pod 8)
func TestBuildShardStatus_3x2_replicaMapping(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 2

	got := buildShardStatus(vc)
	cases := [][]string{
		{"vk-3", "vk-6"},
		{"vk-4", "vk-7"},
		{"vk-5", "vk-8"},
	}
	for i, want := range cases {
		if !reflect.DeepEqual(got[i].ReplicaPods, want) {
			t.Errorf("shard %d replicas: got %v want %v", i, got[i].ReplicaPods, want)
		}
	}
}

func TestBuildShardStatus_zeroShards(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 0
	if got := buildShardStatus(vc); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

// buildShardStatusFromNodes — NODES 응답 기반 ShardStatus.
// 정상 부트스트랩 직후 토폴로지 검증.
func TestBuildShardStatusFromNodes_3x1(t *testing.T) {
	nodes := []vk.NodeView{
		// shard 0 primary + replica.
		{ID: "p0", Addr: "10.0.0.1:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 0, End: 5460}}},
		// shard 1 primary + replica.
		{ID: "p1", Addr: "10.0.0.2:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 5461, End: 10921}}},
		// shard 2 primary + replica.
		{ID: "p2", Addr: "10.0.0.3:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 10922, End: 16383}}},
		{ID: "r0", Addr: "10.0.0.4:6379", Flags: map[string]bool{"slave": true}, MasterID: "p0"},
		{ID: "r1", Addr: "10.0.0.5:6379", Flags: map[string]bool{"replica": true}, MasterID: "p1"},
		{ID: "r2", Addr: "10.0.0.6:6379", Flags: map[string]bool{"slave": true}, MasterID: "p2"},
	}
	got := buildShardStatusFromNodes(nodes, nil)
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	wantPrimary := []string{"10.0.0.1:6379", "10.0.0.2:6379", "10.0.0.3:6379"}
	wantReplicas := [][]string{
		{"10.0.0.4:6379"}, {"10.0.0.5:6379"}, {"10.0.0.6:6379"},
	}
	wantSlots := []int32{5461, 5461, 5462}
	wantRanges := []string{"0-5460", "5461-10921", "10922-16383"}
	for i, sh := range got {
		if sh.PrimaryPod != wantPrimary[i] {
			t.Errorf("shard %d primary: got %s want %s", i, sh.PrimaryPod, wantPrimary[i])
		}
		if !reflect.DeepEqual(sh.ReplicaPods, wantReplicas[i]) {
			t.Errorf("shard %d replicas: got %v want %v", i, sh.ReplicaPods, wantReplicas[i])
		}
		if sh.AssignedSlots != wantSlots[i] {
			t.Errorf("shard %d slots: got %d want %d", i, sh.AssignedSlots, wantSlots[i])
		}
		if sh.SlotRange != wantRanges[i] {
			t.Errorf("shard %d range: got %s want %s", i, sh.SlotRange, wantRanges[i])
		}
	}
}

// defect ⑥: ValkeyClusterSpec.Modules → cluster STS 에 module init-container +
// --loadmodule 적용. cluster reconcile 이 STSParams.Modules = vc.Spec.Modules 로
// 전달하므로, cluster mode STS 빌드가 모듈을 반영하는지 검증.
func TestClusterSTS_modulesWired(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 1
	vc.Spec.Modules = []cachev1alpha1.ModuleSpec{{Name: "valkey-search"}}

	sts := resources.BuildStatefulSet(resources.STSParams{
		CRName:      vc.Name,
		Namespace:   vc.Namespace,
		Replicas:    vc.Spec.TotalNodes(),
		Image:       "valkey:9",
		ClusterMode: true,
		Modules:     vc.Spec.Modules, // reconcile 의 STSParams 와 동일 wiring.
	})
	ps := sts.Spec.Template.Spec
	if len(ps.InitContainers) != 1 {
		t.Fatalf("cluster STS module init-container 1 기대, got %d", len(ps.InitContainers))
	}
	hasLoad := false
	for _, a := range ps.Containers[0].Args {
		if a == "--loadmodule" {
			hasLoad = true
		}
	}
	if !hasLoad {
		t.Fatalf("cluster STS valkey container args 에 --loadmodule 기대: %v", ps.Containers[0].Args)
	}
}

// defect ④: masters-only — TotalNodes() 가 shards 만 (replica 0).
func TestTotalNodes_mastersOnly(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Spec.Shards = 3
	vc.Spec.ReplicasPerShard = 0
	if got := vc.Spec.TotalNodes(); got != 3 {
		t.Fatalf("masters-only TotalNodes(): got %d want 3", got)
	}
	// podAddresses 도 shards 개 (replica 주소 없음).
	vc.Name = "vk"
	vc.Namespace = "ns"
	if got := podAddresses(vc); len(got) != 3 {
		t.Fatalf("masters-only podAddresses: got %d want 3", len(got))
	}
}

// decidePhase — phase 결정 우선순위 행렬.
func TestDecidePhase_matrix(t *testing.T) {
	mkVC := func(spec, status string) *cachev1alpha1.ValkeyCluster {
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Spec.Version.Version = spec
		vc.Status.Version = status
		return vc
	}
	mkInfo := func(state string, slots int32) *vk.ClusterInfo {
		return &vk.ClusterInfo{State: state, SlotsAssigned: slots}
	}
	cases := []struct {
		name         string
		vc           *cachev1alpha1.ValkeyCluster
		ready, total int32
		info         *vk.ClusterInfo
		want         cachev1alpha1.ClusterPhase
	}{
		{"version change + rolling", mkVC("8.2.0", "8.1.6"), 3, 6, nil, cachev1alpha1.ClusterPhaseUpgrading},
		{"version change + complete", mkVC("8.2.0", "8.1.6"), 6, 6, mkInfo("ok", 16384), cachev1alpha1.ClusterPhaseRunning},
		{"first reconcile (no status)", mkVC("8.1.6", ""), 0, 6, nil, cachev1alpha1.ClusterPhasePending},
		{"sts rolling", mkVC("8.1.6", "8.1.6"), 3, 6, nil, cachev1alpha1.ClusterPhaseInitializing},
		{"sts ready, cluster not ok", mkVC("8.1.6", "8.1.6"), 6, 6, mkInfo("fail", 0), cachev1alpha1.ClusterPhaseInitializing},
		{"resharding", mkVC("8.1.6", "8.1.6"), 6, 6, mkInfo("ok", 8192), cachev1alpha1.ClusterPhaseResharding},
		{"running", mkVC("8.1.6", "8.1.6"), 6, 6, mkInfo("ok", 16384), cachev1alpha1.ClusterPhaseRunning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decidePhase(tc.vc, tc.ready, tc.total, tc.info)
			if got != tc.want {
				t.Errorf("got %s want %s", got, tc.want)
			}
		})
	}
}

// pod 매핑 함수가 제공되면 ip:port → pod 이름으로 변환.
func TestBuildShardStatusFromNodes_addrMapping(t *testing.T) {
	nodes := []vk.NodeView{
		{ID: "p0", Addr: "10.0.0.1:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 0, End: 16383}}},
		{ID: "r0", Addr: "10.0.0.4:6379", Flags: map[string]bool{"slave": true}, MasterID: "p0"},
	}
	addrToPod := func(addr string) string {
		return map[string]string{
			"10.0.0.1:6379": "vk-0",
			"10.0.0.4:6379": "vk-3",
		}[addr]
	}
	got := buildShardStatusFromNodes(nodes, addrToPod)
	if len(got) != 1 {
		t.Fatalf("want 1 shard, got %d", len(got))
	}
	if got[0].PrimaryPod != "vk-0" {
		t.Errorf("primary: got %s want vk-0", got[0].PrimaryPod)
	}
	if !reflect.DeepEqual(got[0].ReplicaPods, []string{"vk-3"}) {
		t.Errorf("replicas: got %v want [vk-3]", got[0].ReplicaPods)
	}
}

// failover 후 — 옛 primary p0 가 fail, 옛 replica r0 가 primary 로 승격.
// 새 NODES 응답은 r0 가 master flag, p0 (옛 primary) 는 fail / handshake.
func TestBuildShardStatusFromNodes_afterFailover(t *testing.T) {
	nodes := []vk.NodeView{
		{ID: "r0", Addr: "10.0.0.4:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 0, End: 5460}}},
		{ID: "p1", Addr: "10.0.0.2:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 5461, End: 10921}}},
		{ID: "p2", Addr: "10.0.0.3:6379", Flags: map[string]bool{"master": true}, Slots: []vk.SlotRange{{Start: 10922, End: 16383}}},
		{ID: "r1", Addr: "10.0.0.5:6379", Flags: map[string]bool{"slave": true}, MasterID: "p1"},
		{ID: "r2", Addr: "10.0.0.6:6379", Flags: map[string]bool{"slave": true}, MasterID: "p2"},
	}
	got := buildShardStatusFromNodes(nodes, nil)
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
	// shard 0 의 primary 가 r0 (옛 replica) 로 승격됐음을 정확히 보고해야 함.
	if got[0].PrimaryPod != "10.0.0.4:6379" {
		t.Errorf("post-failover shard 0 primary: got %s want 10.0.0.4:6379", got[0].PrimaryPod)
	}
	if len(got[0].ReplicaPods) != 0 {
		t.Errorf("post-failover shard 0 replicas: got %v want []", got[0].ReplicaPods)
	}
}

// CA bundle 로드 — Secret 미존재 시 fallback.
func TestLoadCABundle_secretMissing(t *testing.T) {
	r := scalePolicyTestReconciler(nil)
	pool, err := loadCABundle(testCtx(), r.Client, "ns", "missing-secret")
	if err != nil {
		t.Fatalf("missing secret should not error: %v", err)
	}
	if pool != nil {
		t.Errorf("missing secret should return nil pool")
	}
}

// CA bundle 로드 — Secret 존재 + 유효 PEM.
func TestLoadCABundle_validPEM(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca-secret", Namespace: "ns"},
		Data:       map[string][]byte{"ca.crt": []byte(testRootCAPEM)},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(caSecret).Build()
	r := &ValkeyClusterReconciler{Client: c, Scheme: scheme}

	pool, err := loadCABundle(testCtx(), r.Client, "ns", "ca-secret")
	if err != nil {
		t.Fatalf("valid PEM: %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestLoadCABundle_invalidPEM(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca-secret", Namespace: "ns"},
		Data:       map[string][]byte{"ca.crt": []byte("not-a-pem")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(caSecret).Build()
	r := &ValkeyClusterReconciler{Client: c, Scheme: scheme}

	_, err := loadCABundle(testCtx(), r.Client, "ns", "ca-secret")
	if err == nil {
		t.Fatal("invalid PEM should error")
	}
}

// TLS RootCAs end-to-end — CustomCert 로 CA Secret 명시 시 RootCAs 가 채워지고
// InsecureSkipVerify=false.
func TestTLSConfigForCluster_customCertLoaded(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-ca", Namespace: "ns"},
		Data:       map[string][]byte{"ca.crt": []byte(testRootCAPEM)},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(caSecret).Build()
	r := &ValkeyClusterReconciler{Client: c, Scheme: scheme}

	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{
		Enabled:    true,
		CustomCert: &cachev1alpha1.CustomCertSpec{SecretName: "vk-ca"},
	}
	got, err := r.tlsConfigForCluster(testCtx(), vc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.RootCAs == nil {
		t.Error("RootCAs should be non-nil with valid CA Secret")
	}
	if got.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false when CA loaded")
	}
}

// 자체 서명 self-signed CA (테스트 전용 — 임의 PEM, 검증용으로만 파싱).
const testRootCAPEM = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`

// applyDefaults — 기본값 멱등성. 두 번 호출해도 동일.
func TestApplyDefaults_idempotent(t *testing.T) {
	r := &ValkeyClusterReconciler{}
	vc := &cachev1alpha1.ValkeyCluster{}
	r.applyDefaults(vc)

	first := vc.Spec
	r.applyDefaults(vc)
	if !reflect.DeepEqual(first, vc.Spec) {
		t.Fatalf("defaulting not idempotent: first=%+v second=%+v", first, vc.Spec)
	}

	// 명시 값 보존.
	vc2 := &cachev1alpha1.ValkeyCluster{}
	vc2.Spec.Shards = 7
	vc2.Spec.NodeTimeoutMillis = 30000
	vc2.Spec.Version.Version = "8.0.0"
	r.applyDefaults(vc2)
	if vc2.Spec.Shards != 7 || vc2.Spec.NodeTimeoutMillis != 30000 || vc2.Spec.Version.Version != "8.0.0" {
		t.Fatalf("defaulting overrode user-set values: %+v", vc2.Spec)
	}
}

// ScalePolicy 가드 — STS 미존재 시 부트스트랩 (preserve=false, pending=nil).
func TestEvaluateScalePolicy_noSTS(t *testing.T) {
	r := scalePolicyTestReconciler(nil)
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	preserve, pending, err := r.evaluateScalePolicy(testCtx(), vc, 6)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if preserve {
		t.Error("preserve should be false when STS missing")
	}
	if pending != nil {
		t.Errorf("pending should be nil: %+v", pending)
	}
}

// 변경 의도 + Deliberate=false → STS replicas 보존.
func TestEvaluateScalePolicy_pendingScale(t *testing.T) {
	current := int32(6)
	r := scalePolicyTestReconciler(&current)
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	preserve, pending, err := r.evaluateScalePolicy(testCtx(), vc, 9)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !preserve {
		t.Error("preserve should be true when scale not deliberate")
	}
	if pending == nil {
		t.Fatal("pending should be set")
	}
	if pending.CurrentReplicas != 6 || pending.DesiredReplicas != 9 {
		t.Errorf("pending values: %+v", pending)
	}
}

// 변경 의도 + Deliberate=true → 즉시 적용.
func TestEvaluateScalePolicy_deliberate(t *testing.T) {
	current := int32(6)
	r := scalePolicyTestReconciler(&current)
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	vc.Spec.ScalePolicy = &cachev1alpha1.ScalePolicy{Deliberate: true}
	preserve, pending, err := r.evaluateScalePolicy(testCtx(), vc, 9)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if preserve {
		t.Error("preserve should be false when Deliberate=true")
	}
	if pending != nil {
		t.Errorf("pending should be nil: %+v", pending)
	}
}

// 변경 없음 → preserve=false, pending=nil.
func TestEvaluateScalePolicy_noChange(t *testing.T) {
	current := int32(6)
	r := scalePolicyTestReconciler(&current)
	vc := &cachev1alpha1.ValkeyCluster{}
	vc.Name = "vk"
	vc.Namespace = "ns"
	preserve, pending, err := r.evaluateScalePolicy(testCtx(), vc, 6)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if preserve {
		t.Error("preserve should be false when no change")
	}
	if pending != nil {
		t.Errorf("pending should be nil: %+v", pending)
	}
}

// tlsConfigForCluster — 명세대로 nil/non-nil 선택. CA Secret 부재 시 InsecureSkipVerify
// fallback. CA Secret 존재 시 RootCAs 설정 + InsecureSkipVerify=false.
func TestTLSConfigForCluster(t *testing.T) {
	r := scalePolicyTestReconciler(nil) // STS 미존재 fake client.
	ctx := testCtx()

	vc := &cachev1alpha1.ValkeyCluster{}
	got, err := r.tlsConfigForCluster(ctx, vc)
	if err != nil {
		t.Fatalf("nil spec: %v", err)
	}
	if got != nil {
		t.Errorf("nil TLS spec → want nil, got %+v", got)
	}

	vc.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}
	got, err = r.tlsConfigForCluster(ctx, vc)
	if err != nil {
		t.Fatalf("disabled: %v", err)
	}
	if got != nil {
		t.Errorf("disabled TLS → want nil, got %+v", got)
	}

	vc.Name = "test"
	vc.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
	got, err = r.tlsConfigForCluster(ctx, vc)
	if err != nil {
		t.Fatalf("enabled (no CA): %v", err)
	}
	if got == nil {
		t.Fatal("enabled TLS → want non-nil")
	}
	wantSN := "test-headless." + vc.Namespace + ".svc"
	if got.ServerName != wantSN {
		t.Errorf("ServerName: got %q want %q", got.ServerName, wantSN)
	}
	if !got.InsecureSkipVerify {
		t.Error("no CA bundle → want InsecureSkipVerify fallback")
	}
	if got.RootCAs != nil {
		t.Error("no CA bundle → RootCAs should be nil")
	}
}
