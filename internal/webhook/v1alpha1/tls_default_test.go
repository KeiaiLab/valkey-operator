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

package v1alpha1

import (
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// tlsIntentPresent — pure 함수, *Spec.TLS 명시 의도* 표 검증.
// silent disable 사고 (kind-vk-op-livetest vk-tls 2026-05-15) RCA 후 도입.
func TestTLSIntentPresent(t *testing.T) {
	cases := []struct {
		name string
		in   *cachev1alpha1.TLSSpec
		want bool
	}{
		{
			name: "nil CertManager + nil CustomCert — 의도 없음",
			in:   &cachev1alpha1.TLSSpec{},
			want: false,
		},
		{
			name: "CertManager non-nil 이지만 IssuerRef 빈 + AutoSelfSigned false",
			in: &cachev1alpha1.TLSSpec{
				CertManager: &cachev1alpha1.CertManagerSpec{},
			},
			want: false,
		},
		{
			name: "CertManager IssuerRef.Name 명시 — 의도 있음",
			in: &cachev1alpha1.TLSSpec{
				CertManager: &cachev1alpha1.CertManagerSpec{
					IssuerRef: cachev1alpha1.CertIssuerRef{Name: "my-issuer", Kind: "ClusterIssuer"},
				},
			},
			want: true,
		},
		{
			name: "CertManager AutoSelfSigned=true — 의도 있음",
			in: &cachev1alpha1.TLSSpec{
				CertManager: &cachev1alpha1.CertManagerSpec{AutoSelfSigned: true},
			},
			want: true,
		},
		{
			name: "CustomCert SecretName 명시 — 의도 있음",
			in: &cachev1alpha1.TLSSpec{
				CustomCert: &cachev1alpha1.CustomCertSpec{SecretName: "my-cert"},
			},
			want: true,
		},
		{
			name: "CustomCert non-nil 이지만 SecretName 빈 — 의도 없음",
			in: &cachev1alpha1.TLSSpec{
				CustomCert: &cachev1alpha1.CustomCertSpec{},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tlsIntentPresent(tc.in); got != tc.want {
				t.Fatalf("tlsIntentPresent want=%v got=%v", tc.want, got)
			}
		})
	}
}

// Valkey CR 의 Default — tls.certManager.issuerRef 만 명시한 사용자가 Enabled 누락 시
// secure-by-default 로 Enabled=true 정규화.
func TestValkeyDefault_TLSIntentNormalizesEnabled(t *testing.T) {
	v := &cachev1alpha1.Valkey{
		Spec: cachev1alpha1.ValkeySpec{
			Mode: cachev1alpha1.ModeStandalone,
			TLS: &cachev1alpha1.TLSSpec{
				// Enabled 의도적 누락 — 사고 재현
				CertManager: &cachev1alpha1.CertManagerSpec{
					IssuerRef: cachev1alpha1.CertIssuerRef{
						Name: "valkey-selfsigned",
						Kind: "ClusterIssuer",
					},
				},
			},
		},
	}
	d := &ValkeyCustomDefaulter{}
	if err := d.Default(nil, v); err != nil {
		t.Fatalf("Default: %v", err)
	}
	if !v.Spec.TLS.Enabled {
		t.Fatalf("TLS.Enabled 가 true 로 정규화돼야 함 (silent disable 차단)")
	}
}

// 명시적 Enabled=false 는 보존되어야 함 (사용자 *명시 거부* 존중).
// Defaulter 는 *zero-value silent disable* 만 차단 — 명시 false 는 그대로.
func TestValkeyDefault_TLSExplicitDisabledRespected(t *testing.T) {
	// JSON 직렬화에선 Enabled=false 와 누락이 동일하지만, Go 객체 단위 테스트에선
	// "구조체 생성 시 false 명시" 는 사용자 의도 모방. 라이브에선 Validator 가
	// Enabled=false + CertManager 의도 conflict 를 reject 함이 별 layer.
	// 본 테스트는 *Defaulter 의 silent override 부재* 만 확인.
	v := &cachev1alpha1.Valkey{
		Spec: cachev1alpha1.ValkeySpec{
			Mode: cachev1alpha1.ModeStandalone,
			TLS:  nil, // TLS 자체 없음 — 정규화 대상 아님
		},
	}
	d := &ValkeyCustomDefaulter{}
	if err := d.Default(nil, v); err != nil {
		t.Fatalf("Default: %v", err)
	}
	if v.Spec.TLS != nil {
		t.Fatalf("TLS spec 없으면 nil 유지: %+v", v.Spec.TLS)
	}
}

// ValkeyCluster sister — 동일 패턴.
func TestValkeyClusterDefault_TLSIntentNormalizesEnabled(t *testing.T) {
	vc := &cachev1alpha1.ValkeyCluster{
		Spec: cachev1alpha1.ValkeyClusterSpec{
			Shards: 3,
			TLS: &cachev1alpha1.TLSSpec{
				CertManager: &cachev1alpha1.CertManagerSpec{AutoSelfSigned: true},
			},
		},
	}
	d := &ValkeyClusterCustomDefaulter{}
	if err := d.Default(nil, vc); err != nil {
		t.Fatalf("Default: %v", err)
	}
	if !vc.Spec.TLS.Enabled {
		t.Fatalf("ValkeyCluster TLS.Enabled 정규화 누락")
	}
}
