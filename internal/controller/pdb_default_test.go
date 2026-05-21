/*
Copyright 2026 Keiailab.
*/

package controller

import (
	"context"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
	"github.com/keiailab/valkey-operator/internal/resources"
)

// fakePDBClient — CDEX-M1 + M2 test helper. scheme = policyv1 + cachev1alpha1.
func fakePDBClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := policyv1.AddToScheme(scheme); err != nil {
		t.Fatalf("policyv1 scheme: %v", err)
	}
	if err := cachev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("cachev1alpha1 scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

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

// ───────────────────────────────────────────────────────────────────────────
// CDEX-M1 EnsurePDBDeleted unit tests (mongodb_controller.go:313 sister pattern).
// ───────────────────────────────────────────────────────────────────────────

func TestEnsurePDBDeleted_NotFound(t *testing.T) {
	c := fakePDBClient(t)
	if err := EnsurePDBDeleted(context.Background(), c, "missing", "default"); err != nil {
		t.Errorf("missing PDB: expected nil (idempotent), got: %v", err)
	}
}

func TestEnsurePDBDeleted_Existing(t *testing.T) {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pdb", Namespace: "default"},
	}
	c := fakePDBClient(t, pdb)
	if err := EnsurePDBDeleted(context.Background(), c, "test-pdb", "default"); err != nil {
		t.Errorf("existing PDB delete: expected nil, got: %v", err)
	}
	got := &policyv1.PodDisruptionBudget{}
	err := c.Get(context.Background(), types.NamespacedName{Name: "test-pdb", Namespace: "default"}, got)
	if !apierrors.IsNotFound(err) {
		t.Errorf("post-delete Get: expected NotFound, got: %v", err)
	}
}

func TestEnsurePDBDeleted_RaceRecovery(t *testing.T) {
	// First call deletes, second call (race after delete) returns nil idempotently.
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "race-pdb", Namespace: "default"},
	}
	c := fakePDBClient(t, pdb)
	_ = EnsurePDBDeleted(context.Background(), c, "race-pdb", "default")
	if err := EnsurePDBDeleted(context.Background(), c, "race-pdb", "default"); err != nil {
		t.Errorf("idempotent re-delete: expected nil, got: %v", err)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// CDEX-M2 BuildShardPDB unit tests (mongodb builder.go:2105 sister pattern).
// ───────────────────────────────────────────────────────────────────────────

func TestBuildShardPDB_default_minAvailable(t *testing.T) {
	// shardReplicas=3 (primary + 2 replica) → default minAvailable=2
	pdb := resources.BuildShardPDB("test", "default", 0, 3, nil)
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.IntVal != 2 {
		t.Errorf("shardReplicas=3: expected MinAvailable=2, got: %v", pdb.Spec.MinAvailable)
	}
	if pdb.Name != "test-shard-0" {
		t.Errorf("expected name=test-shard-0, got: %s", pdb.Name)
	}
}

func TestBuildShardPDB_selector_shard_label(t *testing.T) {
	pdb := resources.BuildShardPDB("test", "default", 2, 3, nil)
	got := pdb.Spec.Selector.MatchLabels[resources.LabelValkeyShard]
	if got != "2" {
		t.Errorf("selector shard label: expected '2', got: '%s'", got)
	}
	if pdb.Spec.Selector.MatchLabels[resources.LabelAppName] != "valkey" {
		t.Error("selector missing app.kubernetes.io/name=valkey")
	}
	if pdb.Spec.Selector.MatchLabels[resources.LabelInstanceName] != "test" {
		t.Error("selector missing app.kubernetes.io/instance=test")
	}
}

func TestBuildShardPDB_minAvailable_floor_1(t *testing.T) {
	// shardReplicas=1 (primary only) → default minAvailable=1 (floor, not 0)
	pdb := resources.BuildShardPDB("solo", "default", 0, 1, nil)
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.IntVal != 1 {
		t.Errorf("shardReplicas=1: expected MinAvailable=1 (floor), got: %v", pdb.Spec.MinAvailable)
	}
}
