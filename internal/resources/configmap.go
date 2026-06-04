/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"bytes"
	"fmt"
	"maps"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/assets"
)

// ConfigData — valkey.conf 템플릿 입력.
type ConfigData struct {
	DataDir                  string
	RequirePass              string
	PersistenceMode          string // RDB | AOF | Both | None
	RDBSchedule              string
	AOFFsync                 string
	ClusterEnabled           bool
	ClusterNodeTimeout       int32
	ClusterReplicaNoFailover bool // Spec.AutoFailover=false 일 때 true → "cluster-replica-no-failover yes".
	ExternalReplicaEnabled   bool
	ExternalReplicaHost      string
	ExternalReplicaPort      int32
	ExternalReplicaPassword  string
	TLSEnabled               bool
	// TLSAuthClients 는 valkey 의 tls-auth-clients 옵션. yes (mTLS 강제) /
	// optional (cert 검증 옵션) / no (server-only TLS, password-only auth).
	// TLSEnabled=false 일 때는 사용 안 됨.
	TLSAuthClients string
	Extra          map[string]string
}

// mergeSlowLog — Spec.SlowLog 의 ThresholdMicros / MaxEntries 를 valkey.conf
// directive 로 변환해 AdditionalConfig 와 합친다 (사용자 직접 설정 우선).
//
//   - slowlog-log-slower-than: 임계값 microsec (0 = 비활성, -1 = 모든 명령)
//   - slowlog-max-len: FIFO 보존 entry 수
//
// AdditionalConfig 에 이미 동일 key 가 있으면 사용자 명시 우선 (override).
func mergeSlowLog(extra map[string]string, sl *cachev1alpha1.SlowLogSpec) map[string]string {
	if sl == nil {
		return extra
	}
	out := make(map[string]string, len(extra)+2)
	if sl.ThresholdMicros != 0 || sl.MaxEntries != 0 {
		if sl.ThresholdMicros != 0 {
			out["slowlog-log-slower-than"] = fmt.Sprintf("%d", sl.ThresholdMicros)
		}
		if sl.MaxEntries != 0 {
			out["slowlog-max-len"] = fmt.Sprintf("%d", sl.MaxEntries)
		}
	}
	maps.Copy(out, extra) // 사용자 명시 우선 (override).
	return out
}

// resolveTLSAuthClients — TLSSpec.ClientAuth 의 enum 값을 valkey.conf 의
// tls-auth-clients 옵션 값으로 변환. default (빈 문자열) = required.
func resolveTLSAuthClients(clientAuth string) string {
	switch clientAuth {
	case "optional":
		return "optional"
	case "disabled":
		return "no"
	default: // required, "" (default)
		return "yes"
	}
}

// RenderValkeyConf — valkey.conf 문자열 렌더링.
func RenderValkeyConf(d ConfigData) (string, error) {
	t, err := template.New("valkey.conf").Parse(assets.ValkeyConfTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// BuildConfigMapForValkey — Valkey CR (Standalone/Replication) 용 ConfigMap.
func BuildConfigMapForValkey(
	vk *cachev1alpha1.Valkey,
	password string,
	externalReplicaPassword ...string,
) (*corev1.ConfigMap, error) {
	extPassword := ""
	if len(externalReplicaPassword) > 0 {
		extPassword = externalReplicaPassword[0]
	}
	d := configDataFromValkey(vk, password, extPassword)
	conf, err := RenderValkeyConf(d)
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(vk.Name),
			Namespace: vk.Namespace,
			Labels:    CommonLabels(vk.Name, "valkey"),
		},
		Data: map[string]string{ConfigFileName: conf},
	}, nil
}

// BuildConfigMapForValkeyCluster — ValkeyCluster CR 용 ConfigMap (cluster-enabled yes).
func BuildConfigMapForValkeyCluster(vc *cachev1alpha1.ValkeyCluster, password string) (*corev1.ConfigMap, error) {
	d := configDataFromCluster(vc, password)
	conf, err := RenderValkeyConf(d)
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(vc.Name),
			Namespace: vc.Namespace,
			Labels:    CommonLabels(vc.Name, "valkey-cluster"),
		},
		Data: map[string]string{ConfigFileName: conf},
	}, nil
}

func configDataFromValkey(vk *cachev1alpha1.Valkey, password, externalReplicaPassword string) ConfigData {
	d := ConfigData{
		DataDir:         DataDir,
		RequirePass:     password,
		PersistenceMode: "RDB",
		RDBSchedule:     "3600 1 300 100 60 10000",
		AOFFsync:        "everysec",
		Extra:           mergeSlowLog(vk.Spec.AdditionalConfig, vk.Spec.SlowLog),
	}
	if vk.Spec.ExternalReplica != nil && vk.Spec.ExternalReplica.Enabled {
		d.ExternalReplicaEnabled = true
		d.ExternalReplicaHost = vk.Spec.ExternalReplica.Host
		d.ExternalReplicaPort = vk.Spec.ExternalReplica.Port
		if d.ExternalReplicaPort == 0 {
			d.ExternalReplicaPort = PortClient
		}
		d.ExternalReplicaPassword = externalReplicaPassword
	}
	if vk.Spec.Persistence != nil {
		if vk.Spec.Persistence.Mode != "" {
			d.PersistenceMode = vk.Spec.Persistence.Mode
		}
		if vk.Spec.Persistence.RDBSaveSchedule != "" {
			d.RDBSchedule = vk.Spec.Persistence.RDBSaveSchedule
		}
		if vk.Spec.Persistence.AOFAppendFsync != "" {
			d.AOFFsync = vk.Spec.Persistence.AOFAppendFsync
		}
	}
	if vk.Spec.TLS != nil && vk.Spec.TLS.Enabled {
		d.TLSEnabled = true
		d.TLSAuthClients = resolveTLSAuthClients(vk.Spec.TLS.ClientAuth)
	}
	return d
}

func configDataFromCluster(vc *cachev1alpha1.ValkeyCluster, password string) ConfigData {
	d := ConfigData{
		DataDir:                  DataDir,
		RequirePass:              password,
		PersistenceMode:          "RDB",
		RDBSchedule:              "3600 1 300 100 60 10000",
		AOFFsync:                 "everysec",
		ClusterEnabled:           true,
		ClusterNodeTimeout:       vc.Spec.NodeTimeoutMillis,
		ClusterReplicaNoFailover: !vc.Spec.AutoFailover,
		Extra:                    mergeSlowLog(vc.Spec.AdditionalConfig, vc.Spec.SlowLog),
	}
	if d.ClusterNodeTimeout == 0 {
		d.ClusterNodeTimeout = 15000
	}
	if vc.Spec.Persistence != nil {
		if vc.Spec.Persistence.Mode != "" {
			d.PersistenceMode = vc.Spec.Persistence.Mode
		}
		if vc.Spec.Persistence.RDBSaveSchedule != "" {
			d.RDBSchedule = vc.Spec.Persistence.RDBSaveSchedule
		}
		if vc.Spec.Persistence.AOFAppendFsync != "" {
			d.AOFFsync = vc.Spec.Persistence.AOFAppendFsync
		}
	}
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		d.TLSEnabled = true
		d.TLSAuthClients = resolveTLSAuthClients(vc.Spec.TLS.ClientAuth)
	}
	return d
}
