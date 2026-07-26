/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package resources

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// 회귀 가드의 근본 이유: 내장 `view` ClusterRole 은 secrets 를 제외하지만
// configmaps 는 get/list/watch 를 허용한다. requirepass / masterauth 가
// ConfigMap 에 렌더되면 읽기전용 계정이 공유 Valkey 마스터 자격을 획득한다.

func TestConfigMap이_평문_password를_담지_않는다(t *testing.T) {
	t.Parallel()
	const pw = "s3cr3t-plaintext-canary"

	t.Run("Valkey(Replication)", func(t *testing.T) {
		t.Parallel()
		vk := &cachev1alpha1.Valkey{}
		vk.Name, vk.Namespace = "rs", "ns"
		cm, err := BuildConfigMapForValkey(vk, pw)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		conf := cm.Data[ConfigFileName]
		if strings.Contains(conf, pw) {
			t.Fatalf("ConfigMap 평문 노출: %s", conf)
		}
		// 주석 텍스트는 무해하므로 *실효 directive 행* 만 검사한다.
		for line := range strings.SplitSeq(conf, "\n") {
			l := strings.TrimSpace(line)
			if strings.HasPrefix(l, "#") {
				continue
			}
			if strings.HasPrefix(l, "requirepass") || strings.HasPrefix(l, "masterauth") {
				t.Fatalf("ConfigMap 에 인증 directive 잔존: %q", l)
			}
		}
		if !strings.Contains(conf, "include "+AuthConfMountPath+"/"+AuthConfFileName) {
			t.Fatalf("include directive 누락: %s", conf)
		}
	})

	t.Run("ValkeyCluster", func(t *testing.T) {
		t.Parallel()
		vc := &cachev1alpha1.ValkeyCluster{}
		vc.Name, vc.Namespace = "vc", "ns"
		cm, err := BuildConfigMapForValkeyCluster(vc, pw)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		conf := cm.Data[ConfigFileName]
		if strings.Contains(conf, pw) {
			t.Fatalf("ConfigMap 평문 노출: %s", conf)
		}
		if !strings.Contains(conf, "include "+AuthConfMountPath+"/"+AuthConfFileName) {
			t.Fatalf("include directive 누락: %s", conf)
		}
	})

	t.Run("auth 미사용(password 빈 문자열)이면 include 도 없다", func(t *testing.T) {
		t.Parallel()
		vk := &cachev1alpha1.Valkey{}
		vk.Name, vk.Namespace = "rs", "ns"
		cm, err := BuildConfigMapForValkey(vk, "")
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if strings.Contains(cm.Data[ConfigFileName], "include ") {
			t.Fatalf("auth 미사용인데 include 렌더: %s", cm.Data[ConfigFileName])
		}
	})
}

func TestBuildAuthConfSecret(t *testing.T) {
	t.Parallel()

	t.Run("requirepass + masterauth 를 auth.conf 키로 담는다", func(t *testing.T) {
		t.Parallel()
		s := BuildAuthConfSecret("rs", "ns", "valkey", "pw1", false, "")
		if s == nil {
			t.Fatal("nil Secret")
		}
		if s.Name != "rs-authconf" || s.Namespace != "ns" {
			t.Errorf("name/ns: %q/%q", s.Name, s.Namespace)
		}
		if s.Type != corev1.SecretTypeOpaque {
			t.Errorf("type: %v", s.Type)
		}
		conf, ok := s.StringData[AuthConfFileName]
		if !ok {
			t.Fatalf("키 %q 누락: %v", AuthConfFileName, s.StringData)
		}
		if !strings.Contains(conf, "requirepass pw1") || !strings.Contains(conf, "masterauth pw1") {
			t.Fatalf("auth directive 누락: %s", conf)
		}
	})

	t.Run("password 없으면 nil (볼륨 미생성)", func(t *testing.T) {
		t.Parallel()
		if s := BuildAuthConfSecret("rs", "ns", "valkey", "", false, ""); s != nil {
			t.Fatalf("password 부재 시 nil 이어야 함: %v", s)
		}
	})

	t.Run("external replica 는 외부 primary password 로 masterauth", func(t *testing.T) {
		t.Parallel()
		conf := RenderAuthConf("mine", true, "theirs")
		if !strings.Contains(conf, "requirepass mine") {
			t.Fatalf("requirepass: %s", conf)
		}
		if !strings.Contains(conf, "masterauth theirs") || strings.Contains(conf, "masterauth mine") {
			t.Fatalf("masterauth 는 외부 primary password 여야 함: %s", conf)
		}
	})

	t.Run("external replica + 외부 password 부재면 masterauth 생략", func(t *testing.T) {
		t.Parallel()
		conf := RenderAuthConf("mine", true, "")
		if strings.Contains(conf, "masterauth") {
			t.Fatalf("무인증 primary 인데 masterauth 렌더: %s", conf)
		}
	})
}

func TestSTS가_authconf_Secret을_readOnly_마운트한다(t *testing.T) {
	t.Parallel()

	sts := BuildStatefulSet(STSParams{
		CRName:             "rs",
		Namespace:          "ns",
		Replicas:           2,
		Image:              "valkey:9",
		AuthConfSecretName: "rs-authconf",
	})

	var found bool
	for _, m := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
		if m.Name == "authconf" {
			found = true
			if m.MountPath != AuthConfMountPath {
				t.Errorf("mountPath: %q", m.MountPath)
			}
			if !m.ReadOnly {
				t.Error("readOnly 여야 한다")
			}
		}
	}
	if !found {
		t.Fatalf("authconf volumeMount 누락: %+v", sts.Spec.Template.Spec.Containers[0].VolumeMounts)
	}

	found = false
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "authconf" {
			found = true
			if v.Secret == nil || v.Secret.SecretName != "rs-authconf" {
				t.Fatalf("Secret 볼륨 소스 불일치: %+v", v)
			}
			if v.Secret.DefaultMode == nil || *v.Secret.DefaultMode != 0o400 {
				t.Errorf("defaultMode 0400 이어야 한다: %+v", v.Secret.DefaultMode)
			}
		}
	}
	if !found {
		t.Fatalf("authconf volume 누락: %+v", sts.Spec.Template.Spec.Volumes)
	}

	// config 볼륨 경로와 겹치지 않아야 한다 (중첩 마운트 회피).
	if strings.HasPrefix(AuthConfMountPath, ConfigMapMountPath+"/") {
		t.Fatalf("authconf 경로가 config 마운트 하위 (%s ⊂ %s)", AuthConfMountPath, ConfigMapMountPath)
	}

	t.Run("AuthConfSecretName 미지정 시 볼륨 없음", func(t *testing.T) {
		t.Parallel()
		s := BuildStatefulSet(STSParams{CRName: "rs", Namespace: "ns", Replicas: 1, Image: "valkey:9"})
		for _, v := range s.Spec.Template.Spec.Volumes {
			if v.Name == "authconf" {
				t.Fatal("미지정인데 authconf 볼륨 생성")
			}
		}
	})
}
