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
package controller

import "testing"

func TestHashAuthSecret_empty(t *testing.T) {
	if h := hashAuthSecret(""); h != "" {
		t.Errorf("empty password should return empty hash, got %q", h)
	}
}

func TestHashAuthSecret_deterministic(t *testing.T) {
	a := hashAuthSecret("secret123")
	b := hashAuthSecret("secret123")
	if a != b || a == "" {
		t.Errorf("deterministic: a=%q b=%q", a, b)
	}
}

func TestHashAuthSecret_different_inputs_different_hash(t *testing.T) {
	a := hashAuthSecret("password-A")
	b := hashAuthSecret("password-B")
	if a == b {
		t.Errorf("different inputs should produce different hashes: %q == %q", a, b)
	}
}

func TestHashAuthSecret_sha256_hex_length(t *testing.T) {
	h := hashAuthSecret("anything")
	if len(h) != 64 { // SHA256 = 32 bytes = 64 hex chars
		t.Errorf("hash length: %d, want 64 hex chars", len(h))
	}
}
