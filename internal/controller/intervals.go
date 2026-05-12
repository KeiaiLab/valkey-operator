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

/*
Copyright 2026 Keiailab.
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
