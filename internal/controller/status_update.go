/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
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
