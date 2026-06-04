/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package assets — embedded valkey config / scripts.
package assets

import _ "embed"

//go:embed valkey.conf.tmpl
var ValkeyConfTemplate string
