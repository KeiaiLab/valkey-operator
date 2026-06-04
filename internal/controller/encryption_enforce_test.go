/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestEncryptionEnforce_audit_path_unchanged_when_encrypted — encryption SC 시
// EncryptionEnforce=true 라도 정상 진행 (audit 통과).
func TestEncryptionEnforce_audit_path_unchanged_when_encrypted(t *testing.T) {
	s := runtime.NewScheme()
	if err := storagev1.AddToScheme(s); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(sc("gp3-enc", "ebs.csi.aws.com",
			map[string]string{"type": "gp3", "encrypted": "true"})).Build()

	enc, _, err := auditEncryptionAtRest(testCtx(), c, "gp3-enc")
	if err != nil || !enc {
		t.Errorf("encrypted SC should pass audit: enc=%v err=%v", enc, err)
	}
}

// TestEncryptionEnforce_audit_detects_unencrypted — 미암호 SC 는 enforce 진입점.
// reconciler 통합은 별도 envtest — 본 단위는 audit helper 의 결과만 검증.
func TestEncryptionEnforce_audit_detects_unencrypted(t *testing.T) {
	s := runtime.NewScheme()
	if err := storagev1.AddToScheme(s); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(s).
		WithObjects(sc("gp3-plain", "ebs.csi.aws.com",
			map[string]string{"type": "gp3"})).Build()

	enc, hint, err := auditEncryptionAtRest(testCtx(), c, "gp3-plain")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if enc {
		t.Error("plain SC should NOT be detected as encrypted")
	}
	if hint == "" {
		t.Error("hint required for enforce error message")
	}
}
