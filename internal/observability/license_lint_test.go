/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// LICENSE 파일 + Chart.yaml 의 license annotation 동기 검증.
//
// 사고 패턴: ArtifactHub / GHCR 의 license metadata 와 실제 LICENSE 파일이
// 다르면 사용자가 *잘못된 license 가정* 으로 코드 사용 → 법적 분쟁 위험.
// LICENSE 파일 자체가 없으면 OSS distribution 의 *기본 권리도 보호 불가*.

package observability

import (
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestLicenseFileExistsAndIsMIT(t *testing.T) {
	candidates := []string{"LICENSE", "../../LICENSE", "../../../LICENSE"}
	var path string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			path = c
			break
		}
	}
	if path == "" {
		t.Fatalf("LICENSE 파일 없음 — OSS distribution 의 법적 보호 부재. %v", candidates)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read LICENSE: %v", err)
	}
	content := string(raw)
	// MIT License 의 표준 식별 markers (프로젝트는 Apache-2.0 → MIT 전환, 커밋 eaebf17/f11e07a).
	must := []string{
		"MIT License",
		"Permission is hereby granted",
		`THE SOFTWARE IS PROVIDED "AS IS"`,
	}
	for _, m := range must {
		if !strings.Contains(content, m) {
			t.Errorf("LICENSE 파일에 MIT 식별자 %q 누락", m)
		}
	}
}

type chartYaml struct {
	Annotations map[string]string `json:"annotations"`
}

func TestChartLicenseAnnotationMatchesLicenseFile(t *testing.T) {
	candidates := []string{"charts/valkey-operator/Chart.yaml", "../../charts/valkey-operator/Chart.yaml", "../../../charts/valkey-operator/Chart.yaml"}
	var chartPath string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			chartPath = c
			break
		}
	}
	if chartPath == "" {
		t.Fatalf("Chart.yaml not found: %v", candidates)
	}
	raw, err := os.ReadFile(chartPath)
	if err != nil {
		t.Fatalf("read Chart.yaml: %v", err)
	}
	var ch chartYaml
	if err := yaml.Unmarshal(raw, &ch); err != nil {
		t.Fatalf("parse Chart.yaml: %v", err)
	}
	got := ch.Annotations["artifacthub.io/license"]
	if got != "MIT" {
		t.Errorf("Chart.yaml annotation artifacthub.io/license=%q (want MIT — LICENSE 파일과 일치)", got)
	}
}
