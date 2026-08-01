/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// 필드 단위 수동 단언은 **빠뜨린 필드를 잡지 못한다** — 실제로 그렇게 놓쳤다:
// conversion_test.go 가 Valkey/ValkeyCluster 를 "테스트하고 있었는데도"
// spec.slowLog 가 Hub(v1alpha2)에 없어 조용히 유실되고 있었고,
// ValkeyBackupTarget 의 gcs/azure 도 마찬가지였다.
//
// 그래서 이 파일은 필드를 **열거하지 않는다**. v1alpha1 객체를 JSON 으로 채운 뒤
// v1alpha2 로 변환하고 되돌려서, 원본 JSON 과 **완전히 같은지**만 본다. 변환이
// JSON byte-copy 이므로 Hub 에 대응 태그가 없는 필드는 왕복에서 사라지고 즉시 실패한다.
// 앞으로 v1alpha1 에 필드가 추가되면 Hub 에 함께 넣지 않는 한 이 테스트가 막는다.
package v1alpha1_test

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/api/v1alpha2"
)

// spoke — v1alpha1 쪽 타입이 만족해야 하는 최소 인터페이스.
type spoke interface {
	ConvertTo(conversion.Hub) error
	ConvertFrom(conversion.Hub) error
}

// assertLosslessRoundTrip — src(JSON 으로 채워진 v1alpha1) → hub → back 왕복 후
// JSON 이 원본과 동일한지 검증한다.
func assertLosslessRoundTrip(t *testing.T, name string, srcJSON string, src, back spoke, hub conversion.Hub) {
	t.Helper()

	if err := json.Unmarshal([]byte(srcJSON), src); err != nil {
		t.Fatalf("%s: 픽스처 unmarshal: %v", name, err)
	}
	if err := src.ConvertTo(hub); err != nil {
		t.Fatalf("%s: ConvertTo: %v", name, err)
	}
	if err := back.ConvertFrom(hub); err != nil {
		t.Fatalf("%s: ConvertFrom: %v", name, err)
	}

	want, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("%s: marshal src: %v", name, err)
	}
	got, err := json.Marshal(back)
	if err != nil {
		t.Fatalf("%s: marshal back: %v", name, err)
	}
	if string(want) != string(got) {
		t.Errorf("%s: 왕복에서 내용이 바뀌었다 — Hub(v1alpha2)에 대응 필드가 없을 가능성이 높다.\n원본: %s\n왕복: %s",
			name, want, got)
	}
}

func TestConversion_전_타입_왕복이_무손실이다(t *testing.T) {
	cases := []struct {
		name    string
		srcJSON string
		src     spoke
		back    spoke
		hub     conversion.Hub
	}{
		{
			name: "Valkey",
			srcJSON: `{"metadata":{"name":"vk","namespace":"ns"},"spec":{
				"mode":"Replication","replicas":3,
				"version":{"version":"8.1.7","image":"docker.io/valkey/valkey"},
				"slowLog":{"thresholdMicros":5000,"maxEntries":256},
				"autoFailover":true}}`,
			src: &v1alpha1.Valkey{}, back: &v1alpha1.Valkey{}, hub: &v1alpha2.Valkey{},
		},
		{
			name: "ValkeyCluster",
			srcJSON: `{"metadata":{"name":"vkc","namespace":"ns"},"spec":{
				"shards":3,"replicasPerShard":0,
				"version":{"version":"8.1.7"},
				"slowLog":{"thresholdMicros":7500,"maxEntries":64}}}`,
			src: &v1alpha1.ValkeyCluster{}, back: &v1alpha1.ValkeyCluster{}, hub: &v1alpha2.ValkeyCluster{},
		},
		{
			name: "ValkeyBackup",
			srcJSON: `{"metadata":{"name":"b","namespace":"ns"},"spec":{
				"clusterRef":{"kind":"ValkeyCluster","name":"vkc"},
				"type":"Full","storageSize":"10Gi","retainPVC":true,"ttl":"168h",
				"volumeSnapshotClassName":"csi-snapclass"}}`,
			src: &v1alpha1.ValkeyBackup{}, back: &v1alpha1.ValkeyBackup{}, hub: &v1alpha2.ValkeyBackup{},
		},
		{
			name: "ValkeyBackupTarget/GCS",
			srcJSON: `{"metadata":{"name":"t-gcs","namespace":"ns"},"spec":{
				"type":"GCS",
				"gcs":{"bucket":"bk","prefix":"c/","credentialsSecretRef":{"name":"s","serviceAccountJSONKey":"key.json"}}}}`,
			src: &v1alpha1.ValkeyBackupTarget{}, back: &v1alpha1.ValkeyBackupTarget{}, hub: &v1alpha2.ValkeyBackupTarget{},
		},
		{
			name: "ValkeyBackupTarget/Azure",
			srcJSON: `{"metadata":{"name":"t-az","namespace":"ns"},"spec":{
				"type":"Azure",
				"azure":{"accountName":"acct","container":"c","prefix":"p/","serviceURL":"https://x",
				"credentialsSecretRef":{"name":"s","accountKeyKey":"AZURE_STORAGE_ACCOUNT_KEY"}}}}`,
			src: &v1alpha1.ValkeyBackupTarget{}, back: &v1alpha1.ValkeyBackupTarget{}, hub: &v1alpha2.ValkeyBackupTarget{},
		},
		{
			name: "ValkeyRestore",
			srcJSON: `{"metadata":{"name":"r","namespace":"ns"},"spec":{
				"clusterRef":{"kind":"ValkeyCluster","name":"vkc"},
				"restoreType":"AOF","pointInTime":"2026-07-31T10:00:00Z",
				"source":{"pvc":{"name":"snap"}}}}`,
			src: &v1alpha1.ValkeyRestore{}, back: &v1alpha1.ValkeyRestore{}, hub: &v1alpha2.ValkeyRestore{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertLosslessRoundTrip(t, tc.name, tc.srcJSON, tc.src, tc.back, tc.hub)
		})
	}
}
