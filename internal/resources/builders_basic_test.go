// 기초 빌더 + 명명 함수 회귀 보호.
// labels.go (StatefulSetName/PDBName/PodFQDN 등), secret.go (GeneratePassword/BuildAuthSecret),
// pdb.go (BuildPDB default + override), service.go (Headless/Client), backup_job.go 명명.

package resources

import (
	"strings"
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

// BuildBackupJob 회귀 보호 (cycle 126) — RDB 복사 Job. plain + TLS 분기,
// password env-var injection, security context (non-root + FSGroup), TTL.
func TestBuildBackupJob(t *testing.T) {
	t.Parallel()
	makeParams := func() BackupJobParams {
		return BackupJobParams{
			BackupName: "nightly",
			Namespace:  "ns",
			PVCName:    "nightly-backup",
			Image:      "ghcr.io/keiailab/valkey-operator:v0.1.0",
			TargetHost: "rs-0.rs-headless.ns.svc",
			TargetPort: 6379,
			PasswordSecret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "rs-auth"},
				Key:                  "password",
			},
		}
	}
	t.Run("plain (non-TLS) job", func(t *testing.T) {
		t.Parallel()
		job := BuildBackupJob(makeParams())
		if job.Name != "nightly-rdb-copy" || job.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", job.Name, job.Namespace)
		}
		c := job.Spec.Template.Spec.Containers[0]
		// command 는 sh -c 형식 + valkey-cli + ls -la 검증.
		if len(c.Command) != 3 || c.Command[0] != "sh" || c.Command[1] != "-c" {
			t.Fatalf("command: %v", c.Command)
		}
		shCmd := c.Command[2]
		if !strings.Contains(shCmd, "valkey-cli") {
			t.Error("valkey-cli 호출 누락")
		}
		if !strings.Contains(shCmd, "$VALKEY_PASSWORD") {
			t.Error("password env-var 사용 누락")
		}
		if !strings.Contains(shCmd, "rs-0.rs-headless.ns.svc") {
			t.Error("target host 누락")
		}
		// plain mode → no TLS volume.
		if len(c.VolumeMounts) != 1 {
			t.Errorf("plain mode VolumeMounts 1 (backup only) 기대, got %d", len(c.VolumeMounts))
		}
	})
	t.Run("TLS adds cert mount", func(t *testing.T) {
		t.Parallel()
		p := makeParams()
		p.UseTLS = true
		p.TLSSecretName = "rs-tls"
		p.TargetPort = 6380
		job := BuildBackupJob(p)
		c := job.Spec.Template.Spec.Containers[0]
		shCmd := c.Command[2]
		if !strings.Contains(shCmd, "--tls") {
			t.Error("TLS mode → --tls flag 누락")
		}
		if !strings.Contains(shCmd, "/tls/ca.crt") {
			t.Error("TLS mode → cert path 누락")
		}
		// TLS mode → 2 VolumeMounts (backup + tls).
		if len(c.VolumeMounts) != 2 {
			t.Errorf("TLS mode VolumeMounts 2 기대, got %d", len(c.VolumeMounts))
		}
		if len(job.Spec.Template.Spec.Volumes) != 2 {
			t.Errorf("TLS mode Volumes 2 기대, got %d", len(job.Spec.Template.Spec.Volumes))
		}
	})
	t.Run("security: non-root + FSGroup 999", func(t *testing.T) {
		t.Parallel()
		job := BuildBackupJob(makeParams())
		sc := job.Spec.Template.Spec.SecurityContext
		if sc == nil {
			t.Fatal("SecurityContext nil")
		}
		if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
			t.Error("RunAsNonRoot=true 기대")
		}
		if sc.FSGroup == nil || *sc.FSGroup != 999 {
			t.Errorf("FSGroup=999 기대, got %v", sc.FSGroup)
		}
	})
	t.Run("Job TTL + BackoffLimit", func(t *testing.T) {
		t.Parallel()
		job := BuildBackupJob(makeParams())
		if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 86400 {
			t.Errorf("TTL 24h (86400) 기대")
		}
		if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 2 {
			t.Errorf("BackoffLimit 2 기대")
		}
	})
}

// BuildConfigMapForValkeyCluster 회귀 보호 (cycle 125) — ValkeyCluster CR 의
// valkey.conf ConfigMap. cluster-enabled yes + cluster-node-timeout default
// 15000ms + autoFailover→cluster-replica-no-failover 분기.
func TestBuildConfigMapForValkeyCluster(t *testing.T) {
	t.Parallel()
	t.Run("default cluster mode + autoFailover=true", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Name = "vc"
		vc.Namespace = "ns"
		vc.Spec.AutoFailover = true
		cm, err := BuildConfigMapForValkeyCluster(vc, "secretpass")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if cm.Name != "vc-config" || cm.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", cm.Name, cm.Namespace)
		}
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "cluster-enabled yes") {
			t.Error("cluster-enabled yes 누락")
		}
		if !strings.Contains(conf, "cluster-node-timeout 15000") {
			t.Error("default cluster-node-timeout 15000ms 누락")
		}
		if !strings.Contains(conf, "requirepass secretpass") {
			t.Error("requirepass injection 누락")
		}
	})
	t.Run("autoFailover=false → cluster-replica-no-failover yes", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Name = "vc"
		vc.Namespace = "ns"
		vc.Spec.AutoFailover = false
		cm, _ := BuildConfigMapForValkeyCluster(vc, "p")
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "cluster-replica-no-failover yes") {
			t.Error("autoFailover=false → cluster-replica-no-failover yes 누락")
		}
	})
	t.Run("custom NodeTimeoutMillis", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Name = "vc"
		vc.Namespace = "ns"
		vc.Spec.NodeTimeoutMillis = 30000
		cm, _ := BuildConfigMapForValkeyCluster(vc, "p")
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "cluster-node-timeout 30000") {
			t.Error("custom NodeTimeoutMillis 30000 누락")
		}
	})
	t.Run("component label = valkey-cluster", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Name = "vc"
		vc.Namespace = "ns"
		cm, _ := BuildConfigMapForValkeyCluster(vc, "p")
		if cm.Labels[LabelComponent] != "valkey-cluster" {
			t.Errorf("component label: %q (want valkey-cluster)", cm.Labels[LabelComponent])
		}
	})
}

// BuildConfigMapForValkey 회귀 보호 (cycle 124) — Valkey CR 의 valkey.conf
// ConfigMap. password injection + persistence mode + TLS 분기.
func TestBuildConfigMapForValkey(t *testing.T) {
	t.Parallel()
	t.Run("default RDB persistence + password", func(t *testing.T) {
		t.Parallel()
		vk := &cachev1alpha1.Valkey{}
		vk.Name = "rs"
		vk.Namespace = "ns"
		cm, err := BuildConfigMapForValkey(vk, "secretpass")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if cm.Name != "rs-config" || cm.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", cm.Name, cm.Namespace)
		}
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "requirepass secretpass") {
			t.Error("requirepass injection 누락")
		}
		if !strings.Contains(conf, "save 3600 1 300 100 60 10000") {
			t.Error("default RDB schedule 누락")
		}
	})
	t.Run("AOF persistence", func(t *testing.T) {
		t.Parallel()
		vk := &cachev1alpha1.Valkey{}
		vk.Name = "rs"
		vk.Namespace = "ns"
		vk.Spec.Persistence = &cachev1alpha1.PersistencePolicy{
			Mode:           "AOF",
			AOFAppendFsync: "always",
		}
		cm, _ := BuildConfigMapForValkey(vk, "p")
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "appendonly yes") {
			t.Error("AOF mode → appendonly yes 누락")
		}
		if !strings.Contains(conf, "appendfsync always") {
			t.Error("AOF fsync override 누락")
		}
	})
	t.Run("TLS enabled adds tls config", func(t *testing.T) {
		t.Parallel()
		vk := &cachev1alpha1.Valkey{}
		vk.Name = "rs"
		vk.Namespace = "ns"
		vk.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
		cm, _ := BuildConfigMapForValkey(vk, "p")
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "tls-port") {
			t.Error("TLS enabled → tls-port directive 누락")
		}
	})
}

// BuildBackupPVC 회귀 보호 (cycle 123) — ValkeyBackup CR 의 결과 PVC.
// default size 8Gi + override + ReadWriteOnce + BackupLabels.
func TestBuildBackupPVC(t *testing.T) {
	t.Parallel()
	t.Run("default 8Gi size", func(t *testing.T) {
		t.Parallel()
		b := &cachev1alpha1.ValkeyBackup{}
		b.Name = "nightly"
		b.Namespace = "ns"
		pvc := BuildBackupPVC(b)
		if pvc.Name != "nightly-backup" || pvc.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", pvc.Name, pvc.Namespace)
		}
		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != "ReadWriteOnce" {
			t.Errorf("accessMode: %v", pvc.Spec.AccessModes)
		}
		got := pvc.Spec.Resources.Requests["storage"]
		if got.String() != "8Gi" {
			t.Errorf("default size 8Gi 기대, got %s", got.String())
		}
	})
	t.Run("override storageSize", func(t *testing.T) {
		t.Parallel()
		b := &cachev1alpha1.ValkeyBackup{}
		b.Name = "big"
		b.Namespace = "ns"
		b.Spec.StorageSize = "100Gi"
		pvc := BuildBackupPVC(b)
		got := pvc.Spec.Resources.Requests["storage"]
		if got.String() != "100Gi" {
			t.Errorf("override 100Gi 기대, got %s", got.String())
		}
	})
	t.Run("BackupLabels applied", func(t *testing.T) {
		t.Parallel()
		b := &cachev1alpha1.ValkeyBackup{}
		b.Name = "test"
		b.Namespace = "ns"
		pvc := BuildBackupPVC(b)
		if pvc.Labels["app.kubernetes.io/instance"] != "test" {
			t.Errorf("instance label: %q", pvc.Labels["app.kubernetes.io/instance"])
		}
		if pvc.Labels["app.kubernetes.io/component"] != "backup" {
			t.Errorf("component label: %q", pvc.Labels["app.kubernetes.io/component"])
		}
	})
}

// joinArgs — 본 함수 는 strings.Join(args, " ") 의 micro-implementation.
// BuildBackupJob 의 sh -c 명령 조합 시 사용.
func TestJoinArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"valkey-cli"}, "valkey-cli"},
		{"multiple", []string{"valkey-cli", "-h", "host", "BGSAVE"}, "valkey-cli -h host BGSAVE"},
		{"with spaces in arg", []string{"foo", "bar baz"}, "foo bar baz"}, // 의도된 단순 join.
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := joinArgs(c.in); got != c.want {
				t.Errorf("joinArgs(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// BuildNetworkPolicy 회귀 보호 (cycle 122) — operator 가 생성하는 *Valkey
// instance 자체* 의 NetworkPolicy. cycle 72 의 chart NetworkPolicy 와 별개
// (operator 자체 vs Valkey CR 의 instance pod).
func TestBuildNetworkPolicy(t *testing.T) {
	t.Parallel()
	t.Run("standalone client port only", func(t *testing.T) {
		t.Parallel()
		np := BuildNetworkPolicy("rs", "ns", false, nil)
		if np.Name != "rs" || np.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", np.Name, np.Namespace)
		}
		if len(np.Spec.Ingress) != 1 {
			t.Fatalf("ingress 1 rule 기대, got %d", len(np.Spec.Ingress))
		}
		ports := np.Spec.Ingress[0].Ports
		if len(ports) != 1 || ports[0].Port.IntValue() != PortClient {
			t.Errorf("client port (%d) 기대", PortClient)
		}
	})
	t.Run("cluster mode adds bus port", func(t *testing.T) {
		t.Parallel()
		np := BuildNetworkPolicy("rs", "ns", true, nil)
		ports := np.Spec.Ingress[0].Ports
		if len(ports) != 2 {
			t.Fatalf("client + cluster-bus 2 ports 기대, got %d", len(ports))
		}
		hasBus := false
		for _, p := range ports {
			if p.Port.IntValue() == PortClusterBus {
				hasBus = true
			}
		}
		if !hasBus {
			t.Error("cluster mode 에 cluster-bus port 누락")
		}
	})
	t.Run("self-peer always present", func(t *testing.T) {
		t.Parallel()
		np := BuildNetworkPolicy("rs", "ns", false, nil)
		from := np.Spec.Ingress[0].From
		if len(from) < 1 || from[0].PodSelector == nil {
			t.Error("self-peer (PodSelector with selectorLabels) 누락")
		}
	})
	t.Run("additional ingress merged", func(t *testing.T) {
		t.Parallel()
		extraPodSel := map[string]string{"app": "client"}
		spec := &cachev1alpha1.NetworkPolicySpec{
			AdditionalIngressFrom: []cachev1alpha1.NetworkPolicyPeer{
				{PodSelector: &extraPodSel},
			},
		}
		np := BuildNetworkPolicy("rs", "ns", false, spec)
		from := np.Spec.Ingress[0].From
		if len(from) != 2 {
			t.Errorf("self + additional 2 peers 기대, got %d", len(from))
		}
	})
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
