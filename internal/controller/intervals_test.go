/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import (
	"testing"
	"time"
)

func TestRequeueCadencePreservesExistingDurations(t *testing.T) {
	t.Parallel()

	if requeueSteady != 30*time.Second {
		t.Fatalf("requeueSteady: want 30s, got %s", requeueSteady)
	}
	if requeueProgress != 5*time.Second {
		t.Fatalf("requeueProgress: want 5s, got %s", requeueProgress)
	}
	if requeueDependencyUnavailable != 15*time.Second {
		t.Fatalf("requeueDependencyUnavailable: want 15s, got %s", requeueDependencyUnavailable)
	}
}
