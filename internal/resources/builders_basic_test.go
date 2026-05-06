// 기초 빌더 + 명명 함수 회귀 보호.
// labels.go (StatefulSetName/PDBName/PodFQDN 등), secret.go (GeneratePassword/BuildAuthSecret),
// pdb.go (BuildPDB default + override), service.go (Headless/Client), backup_job.go 명명.

package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestNamingFunctions(t *testing.T) {
	t.Parallel()
	cr := "my-valkey"
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"StatefulSetName", StatefulSetName(cr), "my-valkey"},
		{"HeadlessServiceName", HeadlessServiceName(cr), "my-valkey-headless"},
		{"ClientServiceName", ClientServiceName(cr), "my-valkey"},
		{"MetricsServiceName", MetricsServiceName(cr), "my-valkey-metrics"},
		{"ConfigMapName", ConfigMapName(cr), "my-valkey-config"},
		{"DefaultSecretName", DefaultSecretName(cr), "my-valkey-auth"},
		{"PDBName", PDBName(cr), "my-valkey"},
		{"NetworkPolicyName", NetworkPolicyName(cr), "my-valkey"},
		{"ServiceMonitorName", ServiceMonitorName(cr), "my-valkey"},
		{"BackupPVCName", BackupPVCName("nightly"), "nightly-backup"},
		{"BackupJobName", BackupJobName("nightly"), "nightly-rdb-copy"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.got != c.want {
				t.Errorf("got %q, want %q", c.got, c.want)
			}
		})
	}
}

func TestPodFQDN(t *testing.T) {
	t.Parallel()
	got := PodFQDN("my-valkey", 0, "cache")
	want := "my-valkey-0.my-valkey-headless.cache.svc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// ordinal 변경.
	if got := PodFQDN("rs", 2, "ns"); got != "rs-2.rs-headless.ns.svc" {
		t.Fatalf("ordinal=2: got %q", got)
	}
}

func TestCommonLabelsAndSelectorLabels(t *testing.T) {
	t.Parallel()
	common := CommonLabels("my-valkey", "valkey")
	want := map[string]string{
		LabelAppName:      "valkey",
		LabelInstanceName: "my-valkey",
		LabelComponent:    "valkey",
		LabelManagedBy:    ManagedByValue,
		LabelPartOf:       PartOfValue,
	}
	for k, v := range want {
		if common[k] != v {
			t.Errorf("CommonLabels[%q] = %q, want %q", k, common[k], v)
		}
	}
	sel := SelectorLabels("my-valkey")
	if len(sel) != 2 {
		t.Errorf("SelectorLabels: 안정 라벨 2개여야 함 (app/instance), got %d", len(sel))
	}
	if sel[LabelComponent] != "" {
		t.Error("SelectorLabels 에 component 가 포함되면 안 됨 (Service selector 안정성)")
	}
}

func TestGeneratePassword(t *testing.T) {
	t.Parallel()
	p1, err := GeneratePassword()
	if err != nil {
		t.Fatalf("GeneratePassword err: %v", err)
	}
	if len(p1) != 64 {
		t.Errorf("hex 길이 64 (32 bytes) 기대, got %d", len(p1))
	}
	p2, _ := GeneratePassword()
	if p1 == p2 {
		t.Error("두 password 가 같음 — randomness 결함")
	}
}

func TestBuildAuthSecret(t *testing.T) {
	t.Parallel()
	s := BuildAuthSecret("rs", "ns", "secret123")
	if s.Name != "rs-auth" || s.Namespace != "ns" {
		t.Errorf("name/ns: %q/%q", s.Name, s.Namespace)
	}
	if s.Type != corev1.SecretTypeOpaque {
		t.Errorf("type: %v", s.Type)
	}
	if got := string(s.Data[SecretPasswordKey]); got != "secret123" {
		t.Errorf("password: got %q", got)
	}
}

func TestBuildPDBDefault(t *testing.T) {
	t.Parallel()
	// spec=nil → minAvailable = replicas-1 = 2.
	pdb := BuildPDB("rs", "ns", 3, nil)
	if pdb.Spec.MaxUnavailable != nil {
		t.Error("default 는 minAvailable 을 채워야 함")
	}
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.IntValue() != 2 {
		t.Errorf("minAvailable=2 기대, got %v", pdb.Spec.MinAvailable)
	}
	// replicas=1 → min=1 (clamp 0 → 1).
	pdb1 := BuildPDB("rs", "ns", 1, nil)
	if pdb1.Spec.MinAvailable.IntValue() != 1 {
		t.Errorf("clamp: 1 기대, got %v", pdb1.Spec.MinAvailable)
	}
}

func TestBuildPDBMaxUnavailableOverride(t *testing.T) {
	t.Parallel()
	mu := intstr.FromInt(1)
	spec := &cachev1alpha1.PodDisruptionBudgetSpec{MaxUnavailable: &mu}
	pdb := BuildPDB("rs", "ns", 3, spec)
	if pdb.Spec.MaxUnavailable == nil || pdb.Spec.MaxUnavailable.IntValue() != 1 {
		t.Errorf("MaxUnavailable=1 기대")
	}
	if pdb.Spec.MinAvailable != nil {
		t.Error("MaxUnavailable 지정 시 MinAvailable 이 비어야 함")
	}
}

func TestBuildPDBMinAvailableOverride(t *testing.T) {
	t.Parallel()
	ma := intstr.FromString("50%")
	spec := &cachev1alpha1.PodDisruptionBudgetSpec{MinAvailable: &ma}
	pdb := BuildPDB("rs", "ns", 3, spec)
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.StrVal != "50%" {
		t.Errorf("MinAvailable=50%% 기대")
	}
}

func TestBuildHeadlessService(t *testing.T) {
	t.Parallel()
	// non-cluster: client port 1개.
	svc := BuildHeadlessService("rs", "ns", false)
	if svc.Spec.ClusterIP != "None" {
		t.Error("Headless 는 ClusterIP=None")
	}
	if !svc.Spec.PublishNotReadyAddresses {
		t.Error("PublishNotReadyAddresses=true 여야 함 (cluster init DNS)")
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Name != "client" {
		t.Errorf("non-cluster: client 포트만, got %d ports", len(svc.Spec.Ports))
	}
	// cluster: client + cluster-bus.
	svcC := BuildHeadlessService("rs", "ns", true)
	if len(svcC.Spec.Ports) != 2 {
		t.Errorf("cluster: 2 포트 기대, got %d", len(svcC.Spec.Ports))
	}
	hasBus := false
	for _, p := range svcC.Spec.Ports {
		if p.Name == "cluster-bus" && p.Port == PortClusterBus {
			hasBus = true
		}
	}
	if !hasBus {
		t.Error("cluster mode 에서 cluster-bus 포트 누락")
	}
}

func TestBuildClientService(t *testing.T) {
	t.Parallel()
	svc := BuildClientService("rs", "ns")
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("type: %v", svc.Spec.Type)
	}
	if svc.Name != "rs" {
		t.Errorf("name: %q", svc.Name)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != PortClient {
		t.Error("client 포트 1개여야 함")
	}
}

func TestBuildMetricsService(t *testing.T) {
	t.Parallel()
	svc := BuildMetricsService("rs", "ns")
	if svc.Name != "rs-metrics" {
		t.Errorf("name: %q", svc.Name)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != PortMetrics {
		t.Errorf("metrics 포트 %d 기대", PortMetrics)
	}
	// ServiceMonitor selector 호환성 — labels 가 component=valkey-metrics.
	if svc.Labels[LabelComponent] != "valkey-metrics" {
		t.Errorf("component label: %q", svc.Labels[LabelComponent])
	}
	// MetricsServiceLabels 와 동일.
	mlabels := MetricsServiceLabels("rs")
	if mlabels[LabelComponent] != "valkey-metrics" {
		t.Error("MetricsServiceLabels 가 svc.Labels 와 동기화되지 않음")
	}
}
