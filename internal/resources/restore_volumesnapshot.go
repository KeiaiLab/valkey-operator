/*
Copyright 2026 Keiailab.
*/

package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// SnapshotAPIGroup — VolumeSnapshot CRD 의 API group (PVC.spec.dataSource.apiGroup).
const SnapshotAPIGroup = "snapshot.storage.k8s.io"

// RestoredPVCName — VolumeSnapshot 으로부터 복원된 PVC 의 표준 명명.
// `<restore-name>-restored` — 사용자가 ValkeyRestore CR 만 보고 PVC 추적 가능.
func RestoredPVCName(restoreName string) string { return restoreName + "-restored" }

// BuildPVCFromVolumeSnapshot — VolumeSnapshot 을 dataSource 로 한 PVC CR 생성.
//
// CSI driver 가 snapshot → PVC clone 을 지원해야 함 (대다수 driver 가 지원,
// AWS EBS / Azure Disk / GCE PD / Ceph RBD 모두 지원).
//
// 새 PVC 는 binding 시 CSI driver 가 snapshot 데이터를 새 volume 으로 복사 →
// PVC.status.phase=Bound 도달 시 즉시 valkey 가 마운트 가능.
//
// snapshotName: ValkeyRestore.Spec.Source.VolumeSnapshot.Name
// storageClassName: 새 PVC 용. nil 이면 cluster default 사용.
// size: 새 PVC 의 spec.resources.requests.storage. snapshot 의 원본 PVC 와 같거나 커야 함.
//
// PR #51 (VolumeSnapshot backup) 의 짝.
func BuildPVCFromVolumeSnapshot(restoreName, namespace, snapshotName string,
	storageClassName *string, size resource.Quantity) *corev1.PersistentVolumeClaim {

	if size.IsZero() {
		size = resource.MustParse("8Gi")
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RestoredPVCName(restoreName),
			Namespace: namespace,
			Labels: map[string]string{
				LabelAppName:      "valkey",
				LabelInstanceName: restoreName,
				LabelComponent:    "restore-pvc",
				LabelManagedBy:    ManagedByValue,
				LabelPartOf:       PartOfValue,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: size},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To(SnapshotAPIGroup),
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
			},
		},
	}
	if storageClassName != nil && *storageClassName != "" {
		pvc.Spec.StorageClassName = storageClassName
	}
	return pvc
}

// IsVolumeSnapshotRestore — 본 restore 가 VolumeSnapshot path 인지 판정.
// caller (reconciler) 가 분기에 사용.
func IsVolumeSnapshotRestore(spec *cachev1alpha1.ValkeyRestoreSpec) bool {
	return spec != nil && spec.Source.VolumeSnapshot != nil && spec.Source.VolumeSnapshot.Name != ""
}
