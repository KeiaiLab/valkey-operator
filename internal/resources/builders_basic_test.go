// 기초 빌더 + 명명 함수 회귀 보호.
// labels.go (StatefulSetName/PDBName/PodFQDN 등), secret.go (GeneratePassword/BuildAuthSecret),
// pdb.go (BuildPDB default + override), service.go (Headless/Client), backup_job.go 명명.

package resources

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	svc := BuildHeadlessService("rs", "ns", false, false)
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
	svcC := BuildHeadlessService("rs", "ns", true, false)
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
	svc := BuildClientService("rs", "ns", false)
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("type: %v", svc.Spec.Type)
	}
	if svc.Name != "rs" {
		t.Errorf("name: %q", svc.Name)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != PortClient {
		t.Error("client 포트 1개여야 함")
	}
	policy := corev1.IPFamilyPolicyPreferDualStack
	custom := &cachev1alpha1.ServiceSpec{
		Type:           corev1.ServiceTypeLoadBalancer,
		IPFamilyPolicy: &policy,
		IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
		Annotations: map[string]string{
			"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
		},
		Labels: map[string]string{"traffic": "external"},
	}
	customSvc := BuildClientService("rs", "ns", true, custom)
	if customSvc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Fatalf("custom service type=%s, want LoadBalancer", customSvc.Spec.Type)
	}
	if customSvc.Annotations["service.beta.kubernetes.io/aws-load-balancer-type"] != "nlb" {
		t.Fatalf("service annotations not merged: %v", customSvc.Annotations)
	}
	if customSvc.Labels["traffic"] != "external" {
		t.Fatalf("service labels not merged: %v", customSvc.Labels)
	}
	if customSvc.Spec.IPFamilyPolicy == nil || *customSvc.Spec.IPFamilyPolicy != corev1.IPFamilyPolicyPreferDualStack {
		t.Fatalf("ipFamilyPolicy=%v, want PreferDualStack", customSvc.Spec.IPFamilyPolicy)
	}
	if got := customSvc.Spec.IPFamilies; len(got) != 2 || got[0] != corev1.IPv4Protocol || got[1] != corev1.IPv6Protocol {
		t.Fatalf("ipFamilies=%v, want [IPv4 IPv6]", got)
	}
	if len(customSvc.Spec.Ports) != 2 {
		t.Fatalf("tls enabled client service should expose 2 ports, got %d", len(customSvc.Spec.Ports))
	}
}

// BuildCertificateForValkey + PortIntOrString 회귀 보호 (cycle 128) — 잔여
// pure helper 망라.
func TestBuildCertificateForValkey(t *testing.T) {
	t.Parallel()
	t.Run("nil TLS → nil", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		if got := BuildCertificateForValkey(v); got != nil {
			t.Errorf("nil TLS → expected nil, got %+v", got)
		}
	})
	t.Run("TLS disabled → nil", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: false}
		if got := BuildCertificateForValkey(v); got != nil {
			t.Errorf("Enabled=false → expected nil")
		}
	})
	t.Run("CertManager empty → nil", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.TLS = &cachev1alpha1.TLSSpec{Enabled: true}
		if got := BuildCertificateForValkey(v); got != nil {
			t.Errorf("CertManager nil → expected nil")
		}
	})
	t.Run("empty IssuerRef.Name → nil", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Spec.TLS = &cachev1alpha1.TLSSpec{
			Enabled:     true,
			CertManager: &cachev1alpha1.CertManagerSpec{},
		}
		if got := BuildCertificateForValkey(v); got != nil {
			t.Errorf("empty IssuerRef.Name → expected nil")
		}
	})
	t.Run("valid spec → Certificate object", func(t *testing.T) {
		t.Parallel()
		v := &cachev1alpha1.Valkey{}
		v.Name = "rs"
		v.Namespace = "ns"
		v.Spec.TLS = &cachev1alpha1.TLSSpec{
			Enabled: true,
			CertManager: &cachev1alpha1.CertManagerSpec{
				IssuerRef: cachev1alpha1.CertIssuerRef{Name: "ca-issuer"},
			},
		}
		got := BuildCertificateForValkey(v)
		if got == nil {
			t.Fatal("valid spec → nil 반환")
		}
		// Unstructured object — kind=Certificate.
		if got.GetKind() != "Certificate" {
			t.Errorf("kind: %q (want Certificate)", got.GetKind())
		}
		if got.GetAPIVersion() != "cert-manager.io/v1" {
			t.Errorf("apiVersion: %q", got.GetAPIVersion())
		}
	})
}

// PortIntOrString — intstr.FromInt wrapper. statefulset / service builders 의
// port specification 에 사용.
func TestPortIntOrString(t *testing.T) {
	t.Parallel()
	cases := []int32{6379, 6380, 8080, 16379}
	for _, p := range cases {
		t.Run(fmt.Sprintf("port_%d", p), func(t *testing.T) {
			t.Parallel()
			got := PortIntOrString(p)
			if got.IntValue() != int(p) {
				t.Errorf("PortIntOrString(%d).IntValue() = %d, want %d", p, got.IntValue(), int(p))
			}
		})
	}
}

// BuildStatefulSet 회귀 보호 (cycle 127) — Valkey 의 핵심 STS. 본 builder 의
// regression 은 *전체 operator 동작* 에 영향. 가장 중요한 contract.
func TestBuildStatefulSet(t *testing.T) {
	t.Parallel()
	makeParams := func() STSParams {
		return STSParams{
			CRName:    "rs",
			Namespace: "ns",
			Replicas:  3,
			Image:     "docker.io/valkey/valkey:8.1.6",
			PasswordRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "rs-auth"},
				Key:                  "password",
			},
		}
	}
	t.Run("default standalone", func(t *testing.T) {
		t.Parallel()
		sts := BuildStatefulSet(makeParams())
		if sts.Name != "rs" || sts.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", sts.Name, sts.Namespace)
		}
		if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 3 {
			t.Errorf("replicas 3 기대, got %v", sts.Spec.Replicas)
		}
		if sts.Spec.ServiceName != "rs-headless" {
			t.Errorf("ServiceName 'rs-headless' 기대, got %q", sts.Spec.ServiceName)
		}
		// Selector 안정성 (cycle 24 의 SelectorLabels) — component 라벨 미포함.
		ml := sts.Spec.Selector.MatchLabels
		if _, ok := ml[LabelComponent]; ok {
			t.Error("Selector 에 component 라벨 포함 — Service selector 안정성 위반")
		}
	})
	t.Run("cluster mode adds cluster-bus port", func(t *testing.T) {
		t.Parallel()
		p := makeParams()
		p.ClusterMode = true
		sts := BuildStatefulSet(p)
		c := sts.Spec.Template.Spec.Containers[0]
		hasBus := false
		for _, port := range c.Ports {
			if port.ContainerPort == int32(PortClusterBus) {
				hasBus = true
			}
		}
		if !hasBus {
			t.Error("cluster mode → cluster-bus port (16379) 누락")
		}
	})
	t.Run("password env via SecretKeyRef", func(t *testing.T) {
		t.Parallel()
		sts := BuildStatefulSet(makeParams())
		c := sts.Spec.Template.Spec.Containers[0]
		var hasPwEnv bool
		for _, e := range c.Env {
			if e.Name == "VALKEY_PASSWORD" || e.Name == "REDIS_PASSWORD" {
				if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
					t.Errorf("password env 가 SecretKeyRef 사용 안 함: %+v", e)
				}
				hasPwEnv = true
			}
		}
		if !hasPwEnv {
			t.Error("VALKEY_PASSWORD env 미설정")
		}
	})
	t.Run("default containers satisfy PodSecurity restricted", func(t *testing.T) {
		t.Parallel()
		p := makeParams()
		p.ExporterImg = "oliver006/redis_exporter:latest"
		sts := BuildStatefulSet(p)
		for _, c := range sts.Spec.Template.Spec.Containers {
			if c.SecurityContext == nil {
				t.Fatalf("%s SecurityContext nil", c.Name)
			}
			if c.SecurityContext.AllowPrivilegeEscalation == nil || *c.SecurityContext.AllowPrivilegeEscalation {
				t.Errorf("%s AllowPrivilegeEscalation=false 기대", c.Name)
			}
			if c.SecurityContext.Capabilities == nil || len(c.SecurityContext.Capabilities.Drop) == 0 {
				t.Errorf("%s capabilities.drop=[ALL] 기대", c.Name)
			}
			if c.SecurityContext.SeccompProfile == nil || c.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
				t.Errorf("%s seccompProfile RuntimeDefault 기대", c.Name)
			}
		}
	})
	t.Run("VolumeClaimTemplate persistent storage", func(t *testing.T) {
		t.Parallel()
		sts := BuildStatefulSet(makeParams())
		if len(sts.Spec.VolumeClaimTemplates) != 1 {
			t.Errorf("VolumeClaimTemplate 1 기대 (data PVC), got %d", len(sts.Spec.VolumeClaimTemplates))
		}
		vct := sts.Spec.VolumeClaimTemplates[0]
		if vct.Name != "data" {
			t.Errorf("VolumeClaimTemplate name='data' 기대, got %q", vct.Name)
		}
	})
	t.Run("CloudPirates compatibility pod and storage knobs", func(t *testing.T) {
		t.Parallel()
		rev := int32(7)
		term := int64(45)
		p := makeParams()
		p.RevisionHistoryLimit = &rev
		p.Storage = cachev1alpha1.StorageSpec{
			Ephemeral: true,
			Labels:    map[string]string{"storage-tier": "ephemeral"},
		}
		p.Pod = &cachev1alpha1.PodSpec{
			Labels: map[string]string{"workload": "cache"},
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
			},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull-secret"}},
			HostAliases: []corev1.HostAlias{{
				IP:        "127.0.0.1",
				Hostnames: []string{"rs-0"},
			}},
			ExtraEnv: []corev1.EnvVar{{Name: "CUSTOM_VAR", Value: "custom"}},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{Command: []string{"true"}},
				},
			},
			TerminationGracePeriodSeconds: &term,
		}
		sts := BuildStatefulSet(p)
		if sts.Spec.RevisionHistoryLimit == nil || *sts.Spec.RevisionHistoryLimit != rev {
			t.Fatalf("revisionHistoryLimit=%v, want %d", sts.Spec.RevisionHistoryLimit, rev)
		}
		if len(sts.Spec.VolumeClaimTemplates) != 0 {
			t.Fatalf("ephemeral storage should not create VCT, got %d", len(sts.Spec.VolumeClaimTemplates))
		}
		var hasDataEmptyDir bool
		for _, v := range sts.Spec.Template.Spec.Volumes {
			if v.Name == "data" && v.EmptyDir != nil {
				hasDataEmptyDir = true
			}
		}
		if !hasDataEmptyDir {
			t.Fatal("storage.ephemeral=true should mount data emptyDir")
		}
		tpl := sts.Spec.Template
		if tpl.Labels["workload"] != "cache" {
			t.Errorf("pod label merge 누락: %v", tpl.Labels)
		}
		if tpl.Annotations["prometheus.io/scrape"] != "true" {
			t.Errorf("pod annotation merge 누락: %v", tpl.Annotations)
		}
		spec := tpl.Spec
		if len(spec.ImagePullSecrets) != 1 || spec.ImagePullSecrets[0].Name != "pull-secret" {
			t.Errorf("imagePullSecrets 누락: %v", spec.ImagePullSecrets)
		}
		if len(spec.HostAliases) != 1 || spec.HostAliases[0].IP != "127.0.0.1" {
			t.Errorf("hostAliases 누락: %v", spec.HostAliases)
		}
		if spec.TerminationGracePeriodSeconds == nil || *spec.TerminationGracePeriodSeconds != term {
			t.Errorf("terminationGracePeriodSeconds=%v, want %d", spec.TerminationGracePeriodSeconds, term)
		}
		c := spec.Containers[0]
		if c.StartupProbe == nil {
			t.Fatal("custom startupProbe 누락")
		}
		var hasCustomEnv bool
		for _, env := range c.Env {
			if env.Name == "CUSTOM_VAR" && env.Value == "custom" {
				hasCustomEnv = true
			}
		}
		if !hasCustomEnv {
			t.Errorf("extraEnv 누락: %v", c.Env)
		}
	})
	t.Run("existingClaim uses PVC volume without VCT", func(t *testing.T) {
		t.Parallel()
		p := makeParams()
		p.Storage = cachev1alpha1.StorageSpec{ExistingClaim: "precreated-data"}
		sts := BuildStatefulSet(p)
		if len(sts.Spec.VolumeClaimTemplates) != 0 {
			t.Fatalf("existingClaim should not create VCT, got %d", len(sts.Spec.VolumeClaimTemplates))
		}
		var claim string
		for _, v := range sts.Spec.Template.Spec.Volumes {
			if v.Name == "data" && v.PersistentVolumeClaim != nil {
				claim = v.PersistentVolumeClaim.ClaimName
			}
		}
		if claim != "precreated-data" {
			t.Fatalf("data volume claim=%q, want precreated-data", claim)
		}
	})
	t.Run("exporter sidecar when ExporterImg set", func(t *testing.T) {
		t.Parallel()
		p := makeParams()
		p.ExporterImg = "oliver006/redis_exporter:latest"
		sts := BuildStatefulSet(p)
		containers := sts.Spec.Template.Spec.Containers
		if len(containers) < 2 {
			t.Fatalf("exporter 활성 시 containers ≥ 2 기대, got %d", len(containers))
		}
		var hasExporter bool
		for _, c := range containers {
			if strings.Contains(c.Image, "redis_exporter") || c.Name == "metrics-exporter" {
				hasExporter = true
			}
		}
		if !hasExporter {
			t.Error("exporter sidecar container 누락")
		}
	})
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
	t.Run("ValkeyCluster shards use per-shard RDB paths", func(t *testing.T) {
		t.Parallel()
		p := makeParams()
		p.TargetHosts = []string{
			"vc-0.vc-headless.ns.svc",
			"vc-3.vc-headless.ns.svc",
			"vc-2.vc-headless.ns.svc",
		}
		job := BuildBackupJob(p)
		shCmd := job.Spec.Template.Spec.Containers[0].Command[2]
		for i, host := range p.TargetHosts {
			if !strings.Contains(shCmd, host) {
				t.Fatalf("shard %d host 누락: %s", i, shCmd)
			}
			path := fmt.Sprintf("/backup/shard-%d/dump.rdb", i)
			if !strings.Contains(shCmd, path) {
				t.Fatalf("shard %d RDB path 누락: %s", i, shCmd)
			}
		}
		if !strings.Contains(shCmd, "mkdir -p /backup/shard-0 /backup/shard-1 /backup/shard-2") {
			t.Fatalf("shard directory 생성 명령 누락: %s", shCmd)
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
	t.Run("external replica uses external masterauth instead of local password", func(t *testing.T) {
		t.Parallel()
		vk := &cachev1alpha1.Valkey{}
		vk.Name = "rs"
		vk.Namespace = "ns"
		vk.Spec.ExternalReplica = &cachev1alpha1.ExternalReplicaSpec{
			Enabled: true,
			Host:    "redis-master.example.com",
			Port:    6380,
		}
		cm, _ := BuildConfigMapForValkey(vk, "local-pass", "external-pass")
		conf := cm.Data[ConfigFileName]
		if !strings.Contains(conf, "replicaof redis-master.example.com 6380") {
			t.Fatalf("external replicaof directive 누락: %s", conf)
		}
		if !strings.Contains(conf, "masterauth external-pass") {
			t.Fatalf("external masterauth 누락: %s", conf)
		}
		if strings.Contains(conf, "masterauth local-pass") {
			t.Fatalf("external replica mode 에서 local masterauth 사용 금지: %s", conf)
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
	// iteration 25 (2026-05-07): commons.New 위임 후 *별-rule per source* 패턴.
	// K8s NetworkPolicy 의 ingress rules 는 OR 결합 — 한 rule 에 [self, extra] 합치든
	// 별 rules 로 나누든 *동작 동등*. 본 test 는 rule 개수 비교 대신 *합산 from peers*
	// + *port set* 으로 semantic equivalence 검증.
	allFromPeers := func(np *networkingv1.NetworkPolicy) []networkingv1.NetworkPolicyPeer {
		all := make([]networkingv1.NetworkPolicyPeer, 0, len(np.Spec.Ingress))
		for _, rule := range np.Spec.Ingress {
			all = append(all, rule.From...)
		}
		return all
	}
	allPorts := func(np *networkingv1.NetworkPolicy) map[int]struct{} {
		set := map[int]struct{}{}
		for _, rule := range np.Spec.Ingress {
			for _, p := range rule.Ports {
				set[p.Port.IntValue()] = struct{}{}
			}
		}
		return set
	}

	t.Run("standalone client port only", func(t *testing.T) {
		t.Parallel()
		np := BuildNetworkPolicy("rs", "ns", false, nil)
		if np.Name != "rs" || np.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", np.Name, np.Namespace)
		}
		if len(np.Spec.Ingress) < 1 {
			t.Fatalf("ingress 최소 1 rule 기대, got 0")
		}
		ports := allPorts(np)
		if _, ok := ports[PortClient]; !ok {
			t.Errorf("client port (%d) 누락, ports=%v", PortClient, ports)
		}
		if _, hasBus := ports[PortClusterBus]; hasBus {
			t.Errorf("standalone 에 cluster-bus 포함 안 됨, got bus")
		}
	})
	t.Run("cluster mode adds bus port", func(t *testing.T) {
		t.Parallel()
		np := BuildNetworkPolicy("rs", "ns", true, nil)
		ports := allPorts(np)
		if len(ports) != 2 {
			t.Fatalf("client + cluster-bus 2 ports 기대, got %d (set: %v)", len(ports), ports)
		}
		if _, ok := ports[PortClusterBus]; !ok {
			t.Error("cluster mode 에 cluster-bus port 누락")
		}
	})
	t.Run("self-peer always present", func(t *testing.T) {
		t.Parallel()
		np := BuildNetworkPolicy("rs", "ns", false, nil)
		// self-peer = SelectorLabels(crName) 매칭 PodSelector.
		expected := SelectorLabels("rs")
		var hasSelf bool
		for _, peer := range allFromPeers(np) {
			if peer.PodSelector == nil {
				continue
			}
			match := true
			for k, v := range expected {
				if peer.PodSelector.MatchLabels[k] != v {
					match = false
					break
				}
			}
			if match {
				hasSelf = true
			}
		}
		if !hasSelf {
			t.Error("self-peer (PodSelector with selectorLabels) 누락")
		}
	})
	t.Run("additional ingress merged (semantic — rule split tolerated)", func(t *testing.T) {
		t.Parallel()
		extraPodSel := map[string]string{"app": "client"}
		spec := &cachev1alpha1.NetworkPolicySpec{
			AdditionalIngressFrom: []cachev1alpha1.NetworkPolicyPeer{
				{PodSelector: &extraPodSel},
			},
		}
		np := BuildNetworkPolicy("rs", "ns", false, spec)
		from := allFromPeers(np)
		// self + additional = 2 peers (한 rule 또는 별 rules — semantic 동등).
		if len(from) != 2 {
			t.Errorf("self + additional 합산 2 peers 기대, got %d", len(from))
		}
		// extra peer 의 podSelector 가 추가됐는지 검증.
		var hasExtra bool
		for _, peer := range from {
			if peer.PodSelector != nil && peer.PodSelector.MatchLabels["app"] == "client" {
				hasExtra = true
			}
		}
		if !hasExtra {
			t.Error("additional ingress peer 누락")
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

// TestPodSecurityRestrictedHelper — buildRestrictedContainerSecurityContext()
// helper 가 PodSecurity "restricted" 정책 4 요건을 모두 만족하는지 회귀 가드.
// (1) capabilities.drop=[ALL] (2) seccompProfile.type=RuntimeDefault
// (3) AllowPrivilegeEscalation=false (4) RunAsNonRoot=true.
//
// 본 회귀 가드의 motivation: mongodb-operator 의 동일 결함 (4 곳 인라인
// SecurityContext 의 SeccompProfile 누락) 으로 argos 클러스터에서
// argos-mongo-cfg StatefulSet pod 가 PodSecurity admission 거부되어
// 운영 사고 발생 (2026-05-07). valkey-operator 의 restore / upload /
// download 4 곳 SecurityContext 도 동일 패턴 위반 — fix 적용 + 본 가드.
func TestPodSecurityRestrictedHelper(t *testing.T) {
	sc := buildRestrictedContainerSecurityContext()
	if sc == nil {
		t.Fatal("SecurityContext nil")
	}
	if sc.Capabilities == nil {
		t.Fatal("Capabilities nil")
	}
	hasDropAll := slices.Contains(sc.Capabilities.Drop, "ALL")
	if !hasDropAll {
		t.Errorf("Capabilities.Drop must include ALL, got %v", sc.Capabilities.Drop)
	}
	if sc.SeccompProfile == nil {
		t.Fatal("SeccompProfile nil")
	}
	if sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("SeccompProfile.Type: want RuntimeDefault, got %v", sc.SeccompProfile.Type)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Error("AllowPrivilegeEscalation: want false")
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Error("RunAsNonRoot: want true")
	}
}
