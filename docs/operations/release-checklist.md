# Release Checklist — valkey-operator

신규 release tag (e.g., `v0.1.0`) push 직전에 본 체크리스트를 통과하면
release-grade 품질이 보장된다. *모든 항목* 이 자동화된 게이트로 cover 되어
있어 사람이 수동 검증할 필요 없음 — 본 문서는 *체계 가시화* 용도.

자동화 SSOT: `scripts/release.sh` (수동 실행) + `make gate` + lefthook
pre-push hooks. 본 문서는 *체계 가시화* 용도.

## 1. 빌드 + 코드 품질 (자동)

- [ ] `make lint` — 0 issues (golangci-lint).
- [ ] `make test` — 모든 unit/envtest PASS, 회귀 0건.
- [ ] `make helm-lint` — chart structural valid.
- [ ] `make helm-template` — chart 렌더 valid.
- [ ] `make audit` — govulncheck + gosec + trivy fs (HIGH+CRITICAL fixed = 0).
- [ ] `go build ./...` — 0 errors (모든 OS/arch).

본 6 항목은 lefthook pre-push hook 으로 *모든 push 자동 차단*. release tag
push 도 동일 통과 필수.

## 2. SSOT 동기 게이트 (in-process unit test, internal/observability/)

본 게이트들이 *PR 머지 전* drift 차단:

| # | 게이트 | 검증 |
|---|---|---|
| 1 | `TestADRFilesAllInIndex` | docs/kb/adr/ ↔ INDEX.md 양방향 동기 |
| 2 | `TestADRIndexStatusValid` | Status 컬럼 ∈ {Accepted/Proposed/Deprecated/Superseded by NNNN} |
| 3 | `TestADRIndexSupersededReferencesExist` | Superseded chain 의 모든 참조가 실재 ADR |
| 4 | `TestADRFilesHaveRequiredSections` | 각 ADR 의 Nygard 3 섹션 (Context/Decision/Consequences) |
| 5 | `TestAlertRulesSchemaSanity` | PrometheusRule CRD 스키마 (apiVersion/kind/groups) |
| 6 | `TestAlertRulesAllFieldsValid` | alert × {prefix Valkey + expr + for + severity + annotations} |
| 7 | `TestAlertRulesMetricNamesRegistered` | alert expr 의 valkey_cluster_* metric 이 metrics.go 에 등록됨 |
| 8 | `TestAlertRulesRunbookAnchorsExist` | runbook_url anchor 가 runbook.md 의 GitHub heading 과 매칭 |
| 9 | `TestRBACMarkerResourcesInRole` | kubebuilder:rbac 마커 → role.yaml 양방향 동기 |
| 10 | `TestSamplesStrictUnmarshal` | config/samples/ ↔ api 타입 strict (unknown field 차단) |
| 11 | `TestSamplesDirHasAllExpected` | 등록 매핑 ↔ samples 디렉토리 양방향 |
| 12 | `TestSamplesMetadataValid` | apiVersion/kind/metadata.name 형식 |
| 13 | `TestClusterRefKindEnumMatchesSSOT` | ClusterReference.Kind enum ↔ SSOT 슬라이스 |
| 14 | `TestClusterRefKindAllHaveSwitchCase` | 각 kind 가 controller switch case 보유 |
| 15 | `TestLicenseFileExistsAndIsApache2` | LICENSE 파일 존재 + Apache-2.0 식별자 |
| 16 | `TestChartLicenseAnnotationMatchesLicenseFile` | Chart annotation ↔ LICENSE 파일 |
| 17 | `TestChartImagesAnnotationMatchesAppVersion` | artifacthub.io/images tag ↔ Chart.AppVersion |
| 18 | `TestChartIconURLUsesCurrentValkeyAsset` | Chart icon URL 이 현재 Artifact Hub 에서 fetch 가능한 Valkey logo asset |
| 19 | `TestChartCRDExamplesStrictUnmarshal` | crdsExamples ↔ api 타입 strict |
| 20 | `TestCRDBaseChartSync` | config/crd/bases/ ↔ charts/.../crds/ byte-level (sha256) |
| 21 | `TestChartValuesValkeyVersionMatchesAPIDefault` | values.yaml 의 valkey.version ↔ api default |
| 22 | `TestChartNotesTxtModeValueValidEnum` | NOTES.txt 의 mode: ↔ ValkeyMode enum |
| 23 | `TestChartReadmeYAMLCodeblocksValid` | 전 markdown 의 YAML 블록 mode/apiVersion/kind 검증 (multi-doc) |
| 24 | `TestMarkdownRelativeLinksResolve` | 모든 .md 의 상대 .md link 가 실재 |
| 25 | `TestIssueTemplateReadmeAnchorsExist` | issue template 의 README anchor 실존 |
| 26 | `TestWebhookSetupFunctionsRegisteredInMain` | webhook setup 함수 ↔ main.go 등록 |
| 27 | `TestReconcilerTypesRegisteredInMain` | Reconciler 타입 ↔ main.go 인스턴스화 |
| 28 | `TestRBACRoleResourcesInMarker` | role.yaml 의 resource → kubebuilder:rbac 마커 (orphan rule 차단) |
| 29 | `TestInstallYAMLStructure` | dist/install.yaml 구조 검증 (5 CRD + Deployment + RBAC + Webhook + Service) |
| 30 | `TestKustomizeManifestLabelChainSync` | pod labels ⊇ Deployment selector ⊇ Service selector + ServiceMonitor selector ⊆ Service metadata.labels |
| 31 | `TestKustomizeChartResourcesSync` | config/manager/manager.yaml ↔ charts/.../values.yaml 의 resources (limits + requests × cpu + memory) |
| 32 | `TestKustomizeChartProbesSync` | manager Deployment ↔ chart values.yaml 의 liveness/readiness probe initialDelay/period |
| 33 | `TestKustomizeChartSecurityContextInvariants` | Pod Security Standards "restricted" invariant — runAsNonRoot/seccompProfile/allowPrivilegeEscalation=false/readOnlyRootFilesystem/capabilities.drop=ALL 양쪽 모두 적용 |
| 34 | `TestInstallYAMLOperatorImageEnvMatchesContainerImage` | dist/install.yaml 의 OPERATOR_IMAGE env value ↔ manager 컨테이너 image 일치 (Upload/Download Job ImagePullBackOff 차단) |
| 35 | `TestChartArgsMatchOperatorFlags` | chart deployment + config/manager 의 args ↔ cmd/main.go flag 정의 (옛 flag 잔재 → CrashLoopBackOff 차단) |
| 36 | `TestValuesTemplateBindingCoverage` | values.yaml top-level key 가 templates/ 어디에서든 참조됨 (silent ignore value 차단, 미구현 항목은 exempted + values.yaml 에 명시) |
| 37 | `TestChartFeaturesReconcilerEnvSync` | chart features.{cluster,backup}.enabled ↔ ENABLE_{CLUSTER,BACKUP}_RECONCILER env (operator code 인식) — RBAC + reconciler 정합 (cycle 80 의 helm install default CrashLoopBackOff 차단) |
| 38 | `TestNetworkPolicyWebhookPortPresent` | NetworkPolicy ingress 에 webhook.enabled 조건부 9443 rule (cycles 72/73 cross-feature interaction — webhook silent reject 차단) |
| 39 | `TestNetworkPolicyTracingEgressPresent` | NetworkPolicy egress 에 tracing.endpoint 조건부 OTLP 4317/4318 rule (cycles 65/72 cross-feature — OTEL spans silent loss 차단) |
| 40 | `TestNetworkPolicyBackupEgressPresent` | NetworkPolicy egress 에 features.backup.enabled 조건부 외부 S3 (443/9000) rule (cycles 16/72 cross-feature — BackupTarget Reachable 영구 Pending 차단) |
| 41 | `TestMetricPhaseLabelsSync` | metrics.go::allPhases ↔ api ValkeyPhase + ClusterPhase enum union (Grafana dashboard 의 phase 시계열 incomplete 차단) |
| 42 | `TestGoVersionDockerfileVsGoMod` | Dockerfile 의 FROM golang:X.Y ↔ go.mod 의 `go X.Y` minimum directive 동기 (+ CONTRIBUTING.md Go table — cycle 96) |
| 43 | `TestKubernetesVersionSync` | Chart.yaml kubeVersion ↔ README badge ↔ chart README Kubernetes prerequisite 3-surface 동기 |
| 44 | `TestReleaseTargetInjectsBuildMetadataAndAmd64Only` | release image 의 build metadata 주입 + linux/amd64 단일 platform 강제 (CLAUDE.md §2 멀티아키 금지 정합) |
| 45 | `TestArtifactHubRepositoryMetadataEnablesVerifiedPublisherAndSigningKey` | artifacthub-repo.yml repositoryID + signingKey + owners 유지 |
| 46 | `TestReleasePipelineRequiresSignedHelmCharts` | release/helm-publish 가 기본 HELM_SIGN=1 + .tgz.prov asset 을 강제 |
| 47 | `TestReleaseSmokeVerifiesHelmProvenance` | release-smoke 가 GH Release/gh-pages provenance 와 signingKey 기반 helm verify 검증 |

검증 명령: `go test ./internal/observability/`

## 3. Supply chain (release 시점 강제)

- [ ] `make sbom VERSION=vX.Y.Z` — syft SPDX-2.3 SBOM 생성.
- [ ] release pipeline 이 SBOM 을 GH Release asset 자동 첨부.
- [ ] release pipeline 이 chart `.tgz.prov` 를 GH Release + gh-pages 에 자동 첨부.
- [ ] `bash scripts/release-smoke-test.sh` — 8 단계 (chart asset + SBOM
      asset + Helm provenance verify + helm pull + image manifest + gh-pages +
      trivy CVE scan + cosign verify).
- [ ] 단일 아키텍처 이미지 (linux/amd64 only) — `docker buildx default`
      builder 의 `--platform linux/amd64` 강제 (CLAUDE.md §2 준수).

## 4. 문서

- [ ] CHANGELOG.md `[Unreleased]` 항목 → `[vX.Y.Z]` 로 promote (git-cliff 자동).
- [ ] README §Roadmap 의 "다음 단계" 항목이 실제 release 와 정합.
- [ ] ADR INDEX 동기 (TestADRFilesAllInIndex 가 검증).
- [ ] runbook.md §9 alert 별 대응 — 신규 alert 추가 시 갱신.

## 5. 운영 게이트

- [ ] kubectl 호환성: kubeVersion ≥ 1.26 (Chart.yaml).
- [ ] `make manifests` 결과가 working tree 에 반영됨 (controller-gen drift 0).
      → manifests 타겟이 chart CRD 사본 자동 sync (cycle 38).
- [ ] cert-manager / prometheus-operator 의존 부재 시 graceful fallback
      (NotFound/NoMatch fail-soft).

## 6. 사용자 가시 표면 (자동 검증)

다음이 *모두* OSS 신뢰 지표로 누적:
- LICENSE Apache-2.0 (게이트 #15-16)
- SECURITY.md PGP fingerprint 명시
- CONTRIBUTING.md 환경 요구사항 + PR 절차
- .github/PULL_REQUEST_TEMPLATE.md (게이트 #23 검증)
- .github/ISSUE_TEMPLATE/{bug_report,feature_request,question}.yml (게이트 #24)
- README §Roadmap (게이트 #24 가 anchor 검증)
- ArtifactHub Chart README + crdsExamples (게이트 #17,18,22)
- Issue triage labels (bug/triage 자동)

## 7. release tag push 절차

```bash
# 1. 본 체크리스트의 1-6 자동 게이트 통과 확인
make gate                                # = lint + test + helm + audit
go test ./internal/observability/        # 29 SSOT 게이트
bash scripts/release-smoke-test.sh vX.Y.Z  # 6단계 (image+chart 미배포 시 skip)

# 2. release.sh 수동 실행
bash scripts/release.sh vX.Y.Z

# 3. GH Release 본문 수동 publish (release.sh 가 .release_notes.md 생성)
gh release create vX.Y.Z --notes-file .release_notes.md \
  dist/install.yaml \
  /tmp/valkey-operator-X.Y.Z.spdx.json

# 4. helm-publish (gh-pages → ArtifactHub auto-detect)
make helm-publish HELM_SIGN=1 VERSION=vX.Y.Z
```

## 8. v0.1.0 GA 추가 게이트 (alpha → GA 승격 시)

본 항목은 *현재 alpha 단계 에서는 미적용*, GA tag 시점부터:

- [ ] 24h soak test (kind cluster 장기 가동) — 메모리 leak / FD leak 0.
- [ ] e2e 자동화 (kind + MinIO + ValkeyCluster restore 시나리오) PASS.
- [ ] Track B (Failover/Resharding) ADR + 구현 완료.
- [ ] Conversion Webhook 준비 (v1alpha1 → v1beta1, ADR-0026 deferred).
- [ ] HPA 통합 (Replication mode, ADR-0027 deferred).
- [ ] semver `0.1.0` 또는 `1.0.0` 결정 (사용자 정책).

## 9. 본 체크리스트의 자동 검증

`docs/operations/release-checklist.md` 자체도 SSOT 게이트 안:
- 본 문서가 참조하는 26 게이트가 *실제로 internal/observability/ 에 존재* 하는지
  향후 cycle 의 횡단 게이트로 검증 가능.
- 본 문서의 markdown link 가 broken 이면 `TestMarkdownRelativeLinksResolve` 차단.
