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

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestShouldAutoCreatePDB_nil_spec_replicas_ge_2_yes(t *testing.T) {
	if !shouldAutoCreatePDB(nil, 3) {
		t.Error("nil spec + replicas=3 should auto-create PDB (HA default)")
	}
}

func TestShouldAutoCreatePDB_nil_spec_replicas_1_no(t *testing.T) {
	if shouldAutoCreatePDB(nil, 1) {
		t.Error("nil spec + replicas=1 (Standalone) should NOT create PDB")
	}
}

func TestShouldAutoCreatePDB_explicit_enabled_yes(t *testing.T) {
	spec := &cachev1alpha1.PodDisruptionBudgetSpec{Enabled: true}
	if !shouldAutoCreatePDB(spec, 3) {
		t.Error("explicit Enabled=true should create PDB")
	}
}

func TestShouldAutoCreatePDB_explicit_disabled_opt_out(t *testing.T) {
	spec := &cachev1alpha1.PodDisruptionBudgetSpec{Enabled: false}
	if shouldAutoCreatePDB(spec, 5) {
		t.Error("explicit Enabled=false should be opt-out (no PDB)")
	}
}

func TestShouldAutoCreatePDB_explicit_disabled_replicas_1(t *testing.T) {
	spec := &cachev1alpha1.PodDisruptionBudgetSpec{Enabled: false}
	if shouldAutoCreatePDB(spec, 1) {
		t.Error("explicit Enabled=false + replicas=1 should not create PDB")
	}
}
