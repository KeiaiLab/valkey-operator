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

import (
	appsv1 "k8s.io/api/apps/v1"
)

// appsv1StatefulSet — 작은 wrapper 로 Owns/Get 호출 단순화.
type appsv1StatefulSet struct {
	s appsv1.StatefulSet
}

func (a *appsv1StatefulSet) Inner() *appsv1.StatefulSet { return &a.s }
