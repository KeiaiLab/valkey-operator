/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package controller

import (
	"context"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// updateStatusWithRetry — conflict (ResourceVersion 변경) 시 retry.
func updateStatusWithRetry(ctx context.Context, c client.Client, obj client.Object) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return c.Status().Update(ctx, obj)
	})
}
