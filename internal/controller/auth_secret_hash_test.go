/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuthRotationMark_empty_password(t *testing.T) {
	if m := authRotationMark("", "42"); m != "" {
		t.Errorf("empty password should omit the mark, got %q", m)
	}
}

func TestAuthRotationMark_tracks_secret_version(t *testing.T) {
	a := authRotationMark("secret123", "100")
	b := authRotationMark("secret123", "101")
	if a == b || a == "" {
		t.Errorf("version change must change the mark: a=%q b=%q", a, b)
	}
}

func TestAuthRotationMark_not_derived_from_password(t *testing.T) {
	const pw = "hunter2"
	a := authRotationMark(pw, "7")
	b := authRotationMark("other-password", "7")
	if a != b {
		t.Errorf("mark must not depend on the password: %q != %q", a, b)
	}
	if strings.Contains(a, pw) {
		t.Errorf("mark leaks the password: %q", a)
	}
}

func TestCreatedVersion(t *testing.T) {
	if v := createdVersion(nil); v != "" {
		t.Errorf("nil secret: got %q", v)
	}
	s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "9"}}
	if v := createdVersion(s); v != "9" {
		t.Errorf("created secret: got %q, want 9", v)
	}
}
