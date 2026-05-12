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
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func encryptionScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := storagev1.AddToScheme(s); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return s
}

func sc(name, provisioner string, params map[string]string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		Provisioner: provisioner,
		Parameters:  params,
	}
}

func TestIsLikelyEncrypted_generic_encrypted_true(t *testing.T) {
	enc, _ := isLikelyEncrypted(sc("rook-encrypted", "rook-ceph.rbd.csi.ceph.com",
		map[string]string{"encrypted": "true"}))
	if !enc {
		t.Error("generic encrypted=true should be detected")
	}
}

func TestIsLikelyEncrypted_aws_ebs_encrypted(t *testing.T) {
	enc, _ := isLikelyEncrypted(sc("gp3-enc", "ebs.csi.aws.com",
		map[string]string{"type": "gp3", "encrypted": "true"}))
	if !enc {
		t.Error("AWS EBS encrypted=true should be detected")
	}
}

func TestIsLikelyEncrypted_aws_ebs_no_encryption(t *testing.T) {
	enc, hint := isLikelyEncrypted(sc("gp3-plain", "ebs.csi.aws.com",
		map[string]string{"type": "gp3"}))
	if enc {
		t.Error("AWS EBS without encrypted=true should NOT be detected")
	}
	if hint == "" {
		t.Error("hint should explain why")
	}
}

func TestIsLikelyEncrypted_azure_premium(t *testing.T) {
	enc, _ := isLikelyEncrypted(sc("azure-premium", "disk.csi.azure.com",
		map[string]string{"skuName": "Premium_LRS"}))
	if !enc {
		t.Error("Azure Premium SKU should be detected as encrypted (SSE-D platform)")
	}
}

func TestIsLikelyEncrypted_azure_disk_encryption_set(t *testing.T) {
	enc, _ := isLikelyEncrypted(sc("azure-des", "disk.csi.azure.com",
		map[string]string{"skuName": "Standard_LRS", "diskEncryptionSetID": "/subscriptions/.../des"}))
	if !enc {
		t.Error("Azure with diskEncryptionSetID should be detected")
	}
}

func TestIsLikelyEncrypted_gce_kms(t *testing.T) {
	enc, _ := isLikelyEncrypted(sc("gce-kms", "pd.csi.storage.gke.io",
		map[string]string{"disk-encryption-kms-key": "projects/p/locations/.../keys/k"}))
	if !enc {
		t.Error("GCE PD with KMS key should be detected")
	}
}

func TestIsLikelyEncrypted_unknown_provisioner(t *testing.T) {
	enc, hint := isLikelyEncrypted(sc("custom", "custom-csi.example.com", map[string]string{}))
	if enc {
		t.Error("unknown provisioner should default to not-detected")
	}
	if hint == "" {
		t.Error("hint should mention unknown provisioner")
	}
}

func TestAuditEncryption_empty_storageClassName(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(encryptionScheme(t)).Build()
	enc, hint, err := auditEncryptionAtRest(testCtx(), c, "")
	if err != nil {
		t.Fatalf("empty SC name should not error: %v", err)
	}
	if enc {
		t.Error("empty SC should not claim encrypted")
	}
	if hint == "" {
		t.Error("hint required")
	}
}

func TestAuditEncryption_storageClass_not_found(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(encryptionScheme(t)).Build()
	enc, hint, err := auditEncryptionAtRest(testCtx(), c, "missing")
	if err != nil {
		t.Fatalf("not-found should not propagate err: %v", err)
	}
	if enc {
		t.Error("missing SC should not claim encrypted")
	}
	if hint == "" {
		t.Error("hint should mention not found")
	}
}

func TestAuditEncryption_real_sc_detected(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(encryptionScheme(t)).
		WithObjects(sc("gp3-enc", "ebs.csi.aws.com",
			map[string]string{"type": "gp3", "encrypted": "true"})).Build()
	enc, _, err := auditEncryptionAtRest(testCtx(), c, "gp3-enc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !enc {
		t.Error("real SC with encrypted=true should be detected")
	}
}
