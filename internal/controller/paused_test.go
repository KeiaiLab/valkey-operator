/*
Copyright 2026 Keiailab.

isPaused 헬퍼 단위 테스트 (PausedAnnotation 인식). ADR-0015.
*/

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPaused_nil(t *testing.T) {
	if isPaused(nil) {
		t.Fatalf("nil object — expected not paused")
	}
}

func TestIsPaused_noAnnotation(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "x"}}
	if isPaused(obj) {
		t.Fatalf("no annotations — expected not paused")
	}
}

func TestIsPaused_otherAnnotation(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:        "x",
		Annotations: map[string]string{"foo": "bar"},
	}}
	if isPaused(obj) {
		t.Fatalf("only unrelated annotations — expected not paused")
	}
}

func TestIsPaused_pausedFalse(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:        "x",
		Annotations: map[string]string{PausedAnnotation: "false"},
	}}
	if isPaused(obj) {
		t.Fatalf("paused=false — expected not paused")
	}
}

func TestIsPaused_pausedTrue(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:        "x",
		Annotations: map[string]string{PausedAnnotation: "true"},
	}}
	if !isPaused(obj) {
		t.Fatalf("paused=true — expected paused")
	}
}

// 임의 case-mismatch 는 false 처리 (오타 보호) — annotation 값은 정확히 "true" 만.
func TestIsPaused_caseMismatch(t *testing.T) {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:        "x",
		Annotations: map[string]string{PausedAnnotation: "TRUE"},
	}}
	if isPaused(obj) {
		t.Fatalf("paused=TRUE (case mismatch) — expected not paused")
	}
}
