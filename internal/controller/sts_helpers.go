/*
Copyright 2026 Keiailab.
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
