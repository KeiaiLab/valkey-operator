/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fetchSTSStatus — STS 의 readyReplicas / replicas 조회 (Valkey / ValkeyCluster 공용).
// STS 미존재 시 빈 stsStatus 반환 (생성 전 정상 경로).
func fetchSTSStatus(ctx context.Context, c client.Client, key types.NamespacedName) (*stsStatus, error) {
	sts := &appsv1.StatefulSet{}
	if err := c.Get(ctx, key, sts); err != nil {
		if apierrors.IsNotFound(err) {
			return &stsStatus{}, nil
		}
		return nil, err
	}
	return &stsStatus{
		readyReplicas: sts.Status.ReadyReplicas,
		totalReplicas: sts.Status.Replicas,
	}, nil
}
