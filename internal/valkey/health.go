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
