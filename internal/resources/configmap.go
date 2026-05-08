/*
Copyright 2026 Keiailab.
*/

package resources

import (
	"bytes"
	"fmt"
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
	TLSEnabled               bool
	// TLSAuthClients 는 valkey 의 tls-auth-clients 옵션. yes (mTLS 강제) /
	// optional (cert 검증 옵션) / no (server-only TLS, password-only auth).
	// TLSEnabled=false 일 때는 사용 안 됨.
	TLSAuthClients string
	Extra          map[string]string
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
func BuildConfigMapForValkey(vk *cachev1alpha1.Valkey, password string) (*corev1.ConfigMap, error) {
	d := configDataFromValkey(vk, password)
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

func configDataFromValkey(vk *cachev1alpha1.Valkey, password string) ConfigData {
	d := ConfigData{
		DataDir:         DataDir,
		RequirePass:     password,
		PersistenceMode: "RDB",
		RDBSchedule:     "3600 1 300 100 60 10000",
		AOFFsync:        "everysec",
		Extra:           vk.Spec.AdditionalConfig,
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
		Extra:                    vc.Spec.AdditionalConfig,
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
