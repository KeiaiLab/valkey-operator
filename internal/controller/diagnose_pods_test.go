/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

// 표 기반 — Waiting/Terminated 시그널 → 단일 줄 surface.
func TestFirstUnhealthyContainerMessage(t *testing.T) {
	cases := []struct {
		name      string
		statuses  []corev1.ContainerStatus
		wantEmpty bool
		wantSub   string
	}{
		{
			name:      "empty",
			statuses:  nil,
			wantEmpty: true,
		},
		{
			name: "running healthy",
			statuses: []corev1.ContainerStatus{{
				Name:  "valkey",
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
			wantEmpty: true,
		},
		{
			name: "waiting ContainerCreating (transient, suppressed)",
			statuses: []corev1.ContainerStatus{{
				Name:  "valkey",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			}},
			wantEmpty: true,
		},
		{
			name: "waiting CrashLoopBackOff — terminal",
			statuses: []corev1.ContainerStatus{{
				Name: "valkey",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "back-off 5m0s restarting failed container",
				}},
			}},
			wantSub: "CrashLoopBackOff",
		},
		{
			name: "waiting ImagePullBackOff — terminal",
			statuses: []corev1.ContainerStatus{{
				Name:  "valkey",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "not found"}},
			}},
			wantSub: "ImagePullBackOff",
		},
		{
			name: "terminated exitCode != 0",
			statuses: []corev1.ContainerStatus{{
				Name: "valkey",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1, Message: "Fatal error loading the DB",
				}},
			}},
			wantSub: "exitCode=1",
		},
		{
			name: "terminated exitCode 0 (clean exit, ignored)",
			statuses: []corev1.ContainerStatus{{
				Name:  "valkey",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
			}},
			wantEmpty: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := firstUnhealthyContainerMessage("vk-0", tc.statuses)
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("기대 빈 문자열, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("substring 미포함\n  want substring: %q\n  got: %q", tc.wantSub, got)
			}
			if !strings.Contains(got, "vk-0") {
				t.Fatalf("podName 미포함: %q", got)
			}
		})
	}
}

// diagnoseUnhealthyPods — fake client + 라벨 매칭 pod 1개 CrashLoopBackOff → 메시지 surface.
// vk-bak-target 9 일 stuck (RDB version 80) 사고의 라이브 재현.
func TestDiagnoseUnhealthyPods_surfacesCrashLoopBackOff(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = cachev1alpha1.AddToScheme(scheme)

	v := &cachev1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: "vk-bak-target", Namespace: "default"},
		Spec:       cachev1alpha1.ValkeySpec{Mode: cachev1alpha1.ModeStandalone, Replicas: 1},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-bak-target-0",
			Namespace: "default",
			Labels:    resources.SelectorLabels("vk-bak-target"),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "valkey",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "back-off 5m0s",
				}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1, Message: "Can't handle RDB format version 80",
				}},
			}},
		},
	}
	otherNS := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-bak-target-0",
			Namespace: "other-ns",
			Labels:    resources.SelectorLabels("vk-bak-target"),
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "valkey",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		}}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod, otherNS).Build()
	r := &ValkeyReconciler{Client: c, Scheme: scheme}

	got := r.diagnoseUnhealthyPods(context.Background(), v)
	if !strings.Contains(got, "CrashLoopBackOff") {
		t.Fatalf("CrashLoopBackOff 미surface: %q", got)
	}
	if !strings.Contains(got, "vk-bak-target-0") {
		t.Fatalf("podName 미surface: %q", got)
	}
	if strings.Contains(got, "other-ns") {
		t.Fatalf("namespace 격리 실패 — other-ns pod 가 누출: %q", got)
	}
}

func TestDiagnoseUnhealthyPods_noPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = cachev1alpha1.AddToScheme(scheme)

	v := &cachev1alpha1.Valkey{
		ObjectMeta: metav1.ObjectMeta{Name: "vk", Namespace: "default"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &ValkeyReconciler{Client: c, Scheme: scheme}

	if got := r.diagnoseUnhealthyPods(context.Background(), v); got != "" {
		t.Fatalf("pod 0건일 때 빈 문자열 기대, got %q", got)
	}
}
