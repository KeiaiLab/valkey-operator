/*
Copyright 2026 Keiailab.

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

/*
Copyright 2026 Keiailab.
*/

package controller

import (
	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// 표준 capability 토큰 — Status.Capabilities 슬라이스에 포함될 수 있는 값.
// 변경 시 ValkeyStatus.Capabilities 의 godoc 동기화 의무.
const (
	CapabilityTLS               = "TLS"
	CapabilityTLSAutoCA         = "TLS-AutoCA"
	CapabilityAuth              = "Auth"
	CapabilityAutoscaling       = "Autoscaling"
	CapabilitySlowLog           = "SlowLog"
	CapabilityEncryptionAudit   = "EncryptionAudit"
	CapabilityEncryptionEnforce = "EncryptionEnforce"
	CapabilityNetworkPolicy     = "NetworkPolicy"
	CapabilityMonitoring        = "Monitoring"
	CapabilityExternalReplica   = "ExternalReplica"
	CapabilityEphemeralStorage  = "EphemeralStorage"
)

// AllCapabilities — Prometheus Metric 의 inactive=0 명시 set 위해 전체 리스트
// 노출. 신규 토큰 추가 시 본 슬라이스에도 반영 의무.
var AllCapabilities = []string{
	CapabilityTLS, CapabilityTLSAutoCA, CapabilityAuth, CapabilityAutoscaling,
	CapabilitySlowLog, CapabilityEncryptionAudit, CapabilityEncryptionEnforce,
	CapabilityNetworkPolicy, CapabilityMonitoring, CapabilityExternalReplica,
	CapabilityEphemeralStorage,
}

// computeValkeyCapabilities — Valkey CR 의 활성 optional features 슬라이스 산출.
// reconciler 가 매 reconcile 에서 호출 → Status.Capabilities 갱신.
//
// 결과는 *정렬된 stable order* (UI / kubectl get -o wide 에서 일관 표시).
func computeValkeyCapabilities(v *cachev1alpha1.Valkey) []string {
	out := make([]string, 0, 9)
	if v.Spec.TLS != nil && v.Spec.TLS.Enabled {
		out = append(out, CapabilityTLS)
		if v.Spec.TLS.CertManager != nil && v.Spec.TLS.CertManager.AutoSelfSigned {
			out = append(out, CapabilityTLSAutoCA)
		}
	}
	if v.Spec.Auth.Enabled {
		out = append(out, CapabilityAuth)
	}
	if v.Spec.Autoscaling != nil && v.Spec.Autoscaling.Enabled {
		out = append(out, CapabilityAutoscaling)
	}
	if v.Spec.SlowLog != nil &&
		(v.Spec.SlowLog.ThresholdMicros != 0 || v.Spec.SlowLog.MaxEntries != 0) {
		out = append(out, CapabilitySlowLog)
	}
	if v.Spec.Storage.EncryptionRequired {
		out = append(out, CapabilityEncryptionAudit)
		if v.Spec.Storage.EncryptionEnforce {
			out = append(out, CapabilityEncryptionEnforce)
		}
	}
	if v.Spec.NetworkPolicy != nil && v.Spec.NetworkPolicy.Enabled {
		out = append(out, CapabilityNetworkPolicy)
	}
	if v.Spec.Monitoring != nil && v.Spec.Monitoring.Enabled {
		out = append(out, CapabilityMonitoring)
	}
	if v.Spec.ExternalReplica != nil && v.Spec.ExternalReplica.Enabled {
		out = append(out, CapabilityExternalReplica)
	}
	if v.Spec.Storage.Ephemeral {
		out = append(out, CapabilityEphemeralStorage)
	}
	return out
}

// computeClusterCapabilities — ValkeyCluster CR 동등.
// AutoFailover / SlotMigration 은 cluster 의 *baseline* (default true / Auto)
// 이라 capability 로 노출하지 않음 — 활성 *옵션* 만 강조.
func computeClusterCapabilities(vc *cachev1alpha1.ValkeyCluster) []string {
	out := make([]string, 0, 7)
	if vc.Spec.TLS != nil && vc.Spec.TLS.Enabled {
		out = append(out, CapabilityTLS)
		if vc.Spec.TLS.CertManager != nil && vc.Spec.TLS.CertManager.AutoSelfSigned {
			out = append(out, CapabilityTLSAutoCA)
		}
	}
	if vc.Spec.Auth.Enabled {
		out = append(out, CapabilityAuth)
	}
	if vc.Spec.SlowLog != nil &&
		(vc.Spec.SlowLog.ThresholdMicros != 0 || vc.Spec.SlowLog.MaxEntries != 0) {
		out = append(out, CapabilitySlowLog)
	}
	if vc.Spec.Storage.EncryptionRequired {
		out = append(out, CapabilityEncryptionAudit)
		if vc.Spec.Storage.EncryptionEnforce {
			out = append(out, CapabilityEncryptionEnforce)
		}
	}
	if vc.Spec.NetworkPolicy != nil && vc.Spec.NetworkPolicy.Enabled {
		out = append(out, CapabilityNetworkPolicy)
	}
	if vc.Spec.Monitoring != nil && vc.Spec.Monitoring.Enabled {
		out = append(out, CapabilityMonitoring)
	}
	if vc.Spec.Storage.Ephemeral {
		out = append(out, CapabilityEphemeralStorage)
	}
	return out
}
