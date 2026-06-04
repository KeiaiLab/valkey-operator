/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// CLI sub-command (upload / download) 단위 테스트.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

func TestIsKnownSubcommand(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"upload", true},
		{"download", true},
		{"controller", false},
		{"", false},
		{"foo", false},
	}
	for _, c := range cases {
		if got := IsKnownSubcommand(c.s); got != c.want {
			t.Fatalf("IsKnownSubcommand(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestDispatch_unknown(t *testing.T) {
	err := Dispatch(t.Context(), []string{"foo"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != ErrUnknown {
		t.Fatalf("expected ErrUnknown, got %v", err)
	}
}

func TestDispatch_emptyArgs(t *testing.T) {
	err := Dispatch(t.Context(), nil, &bytes.Buffer{}, &bytes.Buffer{})
	if err != ErrUnknown {
		t.Fatalf("expected ErrUnknown for empty args, got %v", err)
	}
}

// fakeUploader — runUploadWithBuilder mock.
type fakeUploader struct {
	putErr  error
	putSize int64
	host    string
	gotKey  string
	gotPath string
}

func (f *fakeUploader) FPut(_ context.Context, key, path string) (int64, error) {
	f.gotKey = key
	f.gotPath = path
	return f.putSize, f.putErr
}
func (f *fakeUploader) EndpointHost() string { return f.host }

func mockUploadBuilder(c *fakeUploader, buildErr error) clientBuilder {
	return func(_ *cachev1alpha1.S3Spec, _, _ string) (uploader, error) {
		if buildErr != nil {
			return nil, buildErr
		}
		return c, nil
	}
}

func setS3Env(t *testing.T) {
	t.Helper()
	t.Setenv("VALKEY_S3_ENDPOINT", "https://s3.fake")
	t.Setenv("VALKEY_S3_REGION", "us-east-1")
	t.Setenv("VALKEY_S3_ACCESS_KEY_ID", "AKIA")
	t.Setenv("VALKEY_S3_SECRET_ACCESS_KEY", "secret")
}

func TestUpload_success(t *testing.T) {
	setS3Env(t)
	mock := &fakeUploader{putSize: 1024, host: "s3.fake"}
	out := &bytes.Buffer{}
	err := runUploadWithBuilder(t.Context(), uploadOptions{
		bucket:   "b",
		object:   "k",
		filePath: "/tmp/x",
	}, out, mockUploadBuilder(mock, nil))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if mock.gotKey != "k" || mock.gotPath != "/tmp/x" {
		t.Fatalf("FPut got key=%s path=%s", mock.gotKey, mock.gotPath)
	}
	if !strings.Contains(out.String(), "uploaded /tmp/x (1024 bytes)") {
		t.Fatalf("stdout: %s", out.String())
	}
}

func TestUpload_missingFlags(t *testing.T) {
	setS3Env(t)
	err := runUploadWithBuilder(t.Context(), uploadOptions{}, &bytes.Buffer{},
		mockUploadBuilder(&fakeUploader{}, nil))
	if err == nil || !strings.Contains(err.Error(), "all required") {
		t.Fatalf("expected missing flags error, got %v", err)
	}
}

func TestUpload_missingCreds(t *testing.T) {
	t.Setenv("VALKEY_S3_ENDPOINT", "https://s3.fake")
	t.Setenv("VALKEY_S3_REGION", "us-east-1")
	t.Setenv("VALKEY_S3_ACCESS_KEY_ID", "")
	t.Setenv("VALKEY_S3_SECRET_ACCESS_KEY", "")
	err := runUploadWithBuilder(t.Context(), uploadOptions{
		bucket: "b", object: "k", filePath: "/tmp/x",
	}, &bytes.Buffer{}, mockUploadBuilder(&fakeUploader{}, nil))
	if err == nil || !strings.Contains(err.Error(), "VALKEY_S3_ACCESS_KEY_ID") {
		t.Fatalf("expected creds error, got %v", err)
	}
}

func TestUpload_buildFails(t *testing.T) {
	setS3Env(t)
	err := runUploadWithBuilder(t.Context(), uploadOptions{
		bucket: "b", object: "k", filePath: "/tmp/x",
	}, &bytes.Buffer{}, mockUploadBuilder(nil, fmt.Errorf("invalid endpoint")))
	if err == nil || !strings.Contains(err.Error(), "build S3 client") {
		t.Fatalf("expected build error, got %v", err)
	}
}

func TestUpload_fputFails(t *testing.T) {
	setS3Env(t)
	mock := &fakeUploader{putErr: fmt.Errorf("put failed")}
	err := runUploadWithBuilder(t.Context(), uploadOptions{
		bucket: "b", object: "k", filePath: "/tmp/x",
	}, &bytes.Buffer{}, mockUploadBuilder(mock, nil))
	if err == nil || !strings.Contains(err.Error(), "put failed") {
		t.Fatalf("expected upload error, got %v", err)
	}
}

// download 동등 테스트.

type fakeDownloader struct {
	getErr  error
	host    string
	gotKey  string
	gotPath string
}

func (f *fakeDownloader) FGet(_ context.Context, key, path string) error {
	f.gotKey = key
	f.gotPath = path
	return f.getErr
}
func (f *fakeDownloader) EndpointHost() string { return f.host }

func mockDownloadBuilder(c *fakeDownloader, buildErr error) downloadBuilder {
	return func(_ *cachev1alpha1.S3Spec, _, _ string) (downloader, error) {
		if buildErr != nil {
			return nil, buildErr
		}
		return c, nil
	}
}

func TestDownload_success(t *testing.T) {
	setS3Env(t)
	mock := &fakeDownloader{host: "s3.fake"}
	out := &bytes.Buffer{}
	err := runDownloadWithBuilder(t.Context(), downloadOptions{
		bucket: "b", object: "k", filePath: "/tmp/x",
	}, out, mockDownloadBuilder(mock, nil))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if mock.gotKey != "k" || mock.gotPath != "/tmp/x" {
		t.Fatalf("FGet got key=%s path=%s", mock.gotKey, mock.gotPath)
	}
	if !strings.Contains(out.String(), "downloaded s3.fake/k → /tmp/x") {
		t.Fatalf("stdout: %s", out.String())
	}
}

func TestDownload_missingFlags(t *testing.T) {
	setS3Env(t)
	err := runDownloadWithBuilder(t.Context(), downloadOptions{}, &bytes.Buffer{},
		mockDownloadBuilder(&fakeDownloader{}, nil))
	if err == nil || !strings.Contains(err.Error(), "all required") {
		t.Fatalf("expected missing flags error, got %v", err)
	}
}

func TestDownload_fgetFails(t *testing.T) {
	setS3Env(t)
	mock := &fakeDownloader{getErr: fmt.Errorf("get failed")}
	err := runDownloadWithBuilder(t.Context(), downloadOptions{
		bucket: "b", object: "k", filePath: "/tmp/x",
	}, &bytes.Buffer{}, mockDownloadBuilder(mock, nil))
	if err == nil || !strings.Contains(err.Error(), "get failed") {
		t.Fatalf("expected download error, got %v", err)
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("F", "true")
	if !envBool("F") {
		t.Fatal("true should map to true")
	}
	t.Setenv("F", "1")
	if !envBool("F") {
		t.Fatal("1 should map to true")
	}
	t.Setenv("F", "false")
	if envBool("F") {
		t.Fatal("false should map to false")
	}
	t.Setenv("F", "")
	if envBool("F") {
		t.Fatal("empty should map to false")
	}
}
