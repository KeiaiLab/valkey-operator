package observability

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseTargetInjectsBuildMetadataAndMultiArch(t *testing.T) {
	candidates := []string{"Makefile", "../../Makefile", "../../../Makefile"}
	var path string
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		t.Fatalf("Makefile not found: %v", candidates)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(raw)
	for _, want := range []string{
		"--platform linux/amd64,linux/arm64",
		"--build-arg VERSION=\"$(VERSION)\"",
		"--build-arg COMMIT=\"$$COMMIT_VAL\"",
		"--build-arg BUILD_DATE=\"$$DATE_VAL\"",
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("release target 누락: %s", want)
		}
	}
}
