/*
Copyright 2026 Keiailab.

Licensed under the MIT License. See the LICENSE file for details.
*/
package controller

import "time"

const (
	// requeueSteady 는 정상 운영 중 외부 drift 를 주기적으로 회수하는 cadence.
	requeueSteady = 30 * time.Second

	// requeueProgress 는 생성/복구/백업 등 진행 중 상태를 짧게 재확인하는 cadence.
	requeueProgress = 5 * time.Second

	// requeueDependencyUnavailable 는 외부 dependency 가 아직 준비되지 않은 경우의 대기 cadence.
	requeueDependencyUnavailable = 15 * time.Second
)
