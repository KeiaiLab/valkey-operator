/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Finalizer string constants — cross-repo SSoT (B-P0-1).
//
// 외부 사용자 (kubectl jsonpath / ArgoCD finalizer cleanup / Argo Events) 가
// 의존하는 *공개 wire contract*. 변경 시 SemVer major bump + 1 release migration
// window (controller 가 old + new 둘 다 인식) 의무.
//
// 명명 규약 (이미 정합): `<group>.keiailab.io/<kind>-finalizer`. mongodb operator
// 와의 차이: mongodb 는 v1.4 chain 에서 `<kind>.keiailab.com/finalizer` 사용 —
// 향후 v1.5+ 에서 통일 RFC.
package v1alpha2

const (
	// FinalizerValkey — Valkey CR (single-instance / replication) cleanup.
	FinalizerValkey = "cache.keiailab.io/valkey-finalizer"

	// FinalizerValkeyCluster — ValkeyCluster CR (sharded) cleanup. shard
	// statefulset + headless service + cluster bootstrap CR 의무.
	FinalizerValkeyCluster = "cache.keiailab.io/valkeycluster-finalizer"

	// FinalizerValkeyBackup — ValkeyBackup CR cleanup. backup job + S3 storage
	// credential cleanup.
	FinalizerValkeyBackup = "cache.keiailab.io/valkeybackup-finalizer"

	// FinalizerValkeyRestore — ValkeyRestore CR cleanup. restore Pod + init
	// container + ConfigMap cleanup.
	FinalizerValkeyRestore = "cache.keiailab.io/valkeyrestore-finalizer"
)
