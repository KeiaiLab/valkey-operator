/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/

// Package v1alpha1 contains API Schema definitions for the cache v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=cache.keiailab.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	// This name is used by applyconfiguration generators (e.g. controller-gen).
	SchemeGroupVersion = schema.GroupVersion{Group: "cache.keiailab.io", Version: "v1alpha1"}

	// GroupVersion is an alias for SchemeGroupVersion, for backward compatibility.
	GroupVersion = SchemeGroupVersion

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	//
	// scheme.Builder is deprecated upstream (SA1019) in favour of the
	// minimal runtime.SchemeBuilder; migration is tracked across all
	// kubebuilder-generated operators and will land project-wide once
	// the kubebuilder template ships the new pattern.
	//nolint:staticcheck // SA1019: scheme.Builder, awaiting kubebuilder migration
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
