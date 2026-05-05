/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
*/

// Package assets — embedded valkey config / scripts.
package assets

import _ "embed"

//go:embed valkey.conf.tmpl
var ValkeyConfTemplate string
