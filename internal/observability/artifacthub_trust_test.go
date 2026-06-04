/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

package observability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRepoText(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func TestArtifactHubRepositoryMetadataEnablesVerifiedPublisherAndSigningKey(t *testing.T) {
	meta := readRepoText(t, "charts/artifacthub-repo.yml")

	for _, want := range []string{
		"repositoryID: 16085dd0-0f19-4c6b-ab90-bd97105bdf42",
		"signingKey:",
		`fingerprint: "89A409476828CB992338C378651E51AF520BCB78"`,
		"-----BEGIN PGP PUBLIC KEY BLOCK-----",
		"-----END PGP PUBLIC KEY BLOCK-----",
		"owners:",
		"email: support@keiailab.com",
	} {
		if !strings.Contains(meta, want) {
			t.Fatalf("Artifact Hub metadata missing %q", want)
		}
	}
}

func TestReleasePipelineRequiresSignedHelmCharts(t *testing.T) {
	makefile := readRepoText(t, "Makefile")

	for _, want := range []string{
		"HELM_SIGN     ?= 1",
		"HELM_GPG_KEY  ?= Keiailab Helm",
		"HELM_GPG_FINGERPRINT ?= 89A409476828CB992338C378651E51AF520BCB78",
		".PHONY: helm-signing-preflight",
		"helm-signing-preflight:",
		`helm package --sign --key "$(HELM_GPG_KEY)" --keyring "$(HELM_KEYRING)"`,
		"$$PROV_ASSET",
		"valkey-operator-*.tgz.prov",
		"ERROR: signed chart provenance(.tgz.prov) 생성 실패",
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("signed Helm chart release gate missing %q", want)
		}
	}

	for _, forbidden := range []string{
		"HELM_SIGN     ?= 0",
		"HELM_GPG_KEY  ?= 89A409476828CB992338C378651E51AF520BCB78",
	} {
		if strings.Contains(makefile, forbidden) {
			t.Fatalf("unsigned or fingerprint-as-key release default remains: %q", forbidden)
		}
	}
}

func TestReleaseSmokeVerifiesHelmProvenance(t *testing.T) {
	smoke := readRepoText(t, "scripts/release-smoke-test.sh")

	for _, want := range []string{
		"chart .tgz.prov asset 첨부",
		"Artifact Hub Signed badge 불가",
		"helm pull \"${TMP_REPO}/${CHART_NAME}\" --version \"${TAG_VER}\" --destination /tmp --prov",
		"${HELM_REPO_URL}/artifacthub-repo.yml",
		"BEGIN PGP PUBLIC KEY BLOCK",
		`helm verify "$CHART_FILE" --keyring "$PUB_RING"`,
	} {
		if !strings.Contains(smoke, want) {
			t.Fatalf("release smoke provenance gate missing %q", want)
		}
	}
}
