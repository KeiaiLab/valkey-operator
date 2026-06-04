/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package valkey

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// IsClusterReady — cluster_state=ok && slots=16384 && size>=expected.
func IsClusterReady(ctx context.Context, c *redis.Client, expectedSize int32) (bool, *ClusterInfo, error) {
	info, err := QueryClusterInfo(ctx, c)
	if err != nil {
		return false, nil, err
	}
	ready := info.State == "ok" && info.SlotsAssigned == 16384 && info.Size >= expectedSize
	return ready, info, nil
}
