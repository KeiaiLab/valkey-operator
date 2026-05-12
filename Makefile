# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# YEAR defines the year value used for substituting the YEAR placeholder in the boilerplate header.
YEAR ?= $(shell date +%Y)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@echo "=== chart CRD 사본 자동 sync (TestCRDBaseChartSync 게이트 충족) ==="
	@cp config/crd/bases/*.yaml charts/valkey-operator/crds/
	@echo "✓ charts/valkey-operator/crds/ ← config/crd/bases/ 동기 완료"

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt",year=$(YEAR) paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests (combined unit + integration with single cover.out).
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: test-unit
test-unit: fmt vet ## Run unit tests only (no envtest — fast feedback).
	go test -race ./api/... ./internal/observability/... ./internal/storage/... ./internal/cli/... -coverprofile cover-unit.out

.PHONY: test-integration
test-integration: manifests generate fmt vet setup-envtest ## Run integration tests (envtest required).
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test -race ./internal/controller/... ./internal/webhook/... -coverprofile cover-integration.out

.PHONY: coverage-merge
coverage-merge: ## gocovmerge 로 unit + integration coverage 통합 → cover-final.out
	@command -v gocovmerge >/dev/null 2>&1 || { echo "[error] gocovmerge not installed: go install github.com/wadey/gocovmerge@latest"; exit 1; }
	@test -f cover-unit.out || { echo "[error] cover-unit.out 부재 — make test-unit 먼저"; exit 1; }
	@test -f cover-integration.out || { echo "[error] cover-integration.out 부재 — make test-integration 먼저"; exit 1; }
	gocovmerge cover-unit.out cover-integration.out > cover-final.out
	@echo "✓ cover-final.out (unit + integration merged)"
	@go tool cover -func=cover-final.out | tail -1

.PHONY: integration-test
integration-test: fmt vet ## Run real-Valkey integration tests (requires Docker daemon). Tag: integration.
	go test -tags=integration -count=1 -timeout=180s -v ./internal/valkey/...

.PHONY: ssot-check
ssot-check: ## SSOT 게이트 (35+) 만 빠른 검증 (~1s, no envtest). PR 작성 중 iteration.
	@echo "=== SSOT gates only — internal/observability/ ==="
	go test -count=1 -run "^Test" ./internal/observability/
	@echo "✓ all 35+ SSOT gates PASS — release-checklist §2 인벤토리 참조"

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# kubectl kuberc is disabled by default for test isolation; enable with:
# - KUBECTL_KUBERC=true
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= valkey-operator-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

# chaos-mesh manifest URL — ADR-0041 §References. v2.7.x stable (2026-04 기준).
CHAOS_MESH_VERSION ?= v2.7.2

.PHONY: chaos-mesh-install
chaos-mesh-install: ## Install chaos-mesh into the current kubeconfig context (test only).
	kubectl create namespace chaos-mesh --dry-run=client -o yaml | kubectl apply -f -
	curl -sSL https://mirrors.chaos-mesh.org/$(CHAOS_MESH_VERSION)/install.sh | \
		bash -s -- --local kind --name $(KIND_CLUSTER)

.PHONY: chaos-mesh-uninstall
chaos-mesh-uninstall: ## Remove chaos-mesh CRDs + namespace.
	kubectl delete --ignore-not-found namespace chaos-mesh
	kubectl get crd -o name | grep chaos-mesh.org | xargs -r kubectl delete --ignore-not-found

.PHONY: chaos-e2e
chaos-e2e: manifests generate fmt vet ## Run chaos engineering e2e (ADR-0041, requires chaos-mesh installed).
	@echo "==> chaos-e2e: 실행 전 다음이 충족돼야 합니다:"
	@echo "    1. valkey-operator 가 cluster 에 deploy 됨 (make deploy)"
	@echo "    2. chaos-mesh CRD + controller 설치 (make chaos-mesh-install)"
	@echo "    3. namespace=$$CHAOS_TEST_NAMESPACE 에 ValkeyCluster vc-chaos healthy"
	@echo ""
	go test -tags=chaos ./test/chaos/... -v -ginkgo.v -timeout=30m

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary. VERSION 환경변수로 ldflags 주입.
	@VERSION_VAL="$${VERSION:-dev}"; \
	COMMIT_VAL="$$(git rev-parse --short HEAD 2>/dev/null || echo none)"; \
	DATE_VAL="$$(date -u +%Y-%m-%d)"; \
	go build -ldflags "-X main.version=$$VERSION_VAL -X main.commit=$$COMMIT_VAL -X main.date=$$DATE_VAL" \
		-o bin/manager cmd/main.go
	@echo "✓ bin/manager — `bin/manager --version` 으로 확인"

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager (linux/amd64, default builder). VERSION 환경변수로 ldflags 주입.
	# 글로벌 §2: docker buildx 의 기본 빌더 (default) 만 사용. 커스텀 빌더 인스턴스 금지.
	# --platform linux/amd64 명시 — macOS host 에서 native build 시 darwin/arm64 가
	# 되어 cluster (linux) 노드 에 push 시 ImagePullError "no match for platform"
	# (postgres-operator iteration 35 incident 발견). mongodb-operator 패턴 정합.
	@VERSION_VAL="$${VERSION:-dev}"; \
	COMMIT_VAL="$$(git rev-parse --short HEAD 2>/dev/null || echo none)"; \
	DATE_VAL="$$(date -u +%Y-%m-%d)"; \
	docker buildx build --platform linux/amd64 --load \
		--build-arg VERSION=$$VERSION_VAL \
		--build-arg COMMIT=$$COMMIT_VAL \
		--build-arg BUILD_DATE=$$DATE_VAL \
		-t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/amd64
.PHONY: docker-buildx
docker-buildx: ## Build and push multi-arch image. CLAUDE.md §2: default builder + ldflags 자동 주입.
	# CLAUDE.md §2 — default builder 만 사용 (커스텀 builder 인스턴스 금지). buildx
	# default 가 자동 멀티아키 지원. ldflags 자동 주입 (cycle 54 의 docker-build 와
	# 동일 패턴 — production release 의 핵심 경로).
	@VERSION_VAL="$${VERSION:-dev}"; \
	COMMIT_VAL="$$(git rev-parse --short HEAD 2>/dev/null || echo none)"; \
	DATE_VAL="$$(date -u +%Y-%m-%d)"; \
	$(CONTAINER_TOOL) buildx build --push \
		--platform=$(PLATFORMS) \
		--build-arg VERSION=$$VERSION_VAL \
		--build-arg COMMIT=$$COMMIT_VAL \
		--build-arg BUILD_DATE=$$DATE_VAL \
		--tag ${IMG} .

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	# cycle 64: kustomize image transformer 는 container.image 만 갱신 — env
	# OPERATOR_IMAGE 는 별도 sed. dist/install.yaml 의 OPERATOR_IMAGE value 가
	# image: 와 동일하도록 강제 (Upload/Download Job 의 image fallback 차단).
	"$(KUSTOMIZE)" build config/default | \
		sed -E "s|(- name: OPERATOR_IMAGE\n[[:space:]]+value: )controller:latest|\1${IMG}|; s|value: controller:latest|value: ${IMG}|" \
		> dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.20.1

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.11.4
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
	@test -f .custom-gcl.yml && { \
		echo "Building custom golangci-lint with plugins..." && \
		$(GOLANGCI_LINT) custom --destination $(LOCALBIN) --name golangci-lint-custom && \
		mv -f $(LOCALBIN)/golangci-lint-custom $(GOLANGCI_LINT); \
	} || true

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

##@ Release pipeline (RFC 0002 — 100% 로컬, GH Actions 금지)

# Release 변수 — overridable via env or `make release VAR=...`.
IMAGE_REPOSITORY ?= ghcr.io/keiailab/valkey-operator
HELM_CHART       ?= charts/valkey-operator
HELM_REPO_URL    ?= https://keiailab.github.io/valkey-operator
RELEASE_TMP      ?= /tmp/valkey-operator-release
GHPAGES_TMP      ?= /tmp/valkey-operator-gh-pages

.PHONY: require-version
require-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "ERROR: VERSION 필수 (예- make release VERSION=v0.1.0-alpha.1)"; \
		exit 1; \
	fi

.PHONY: helm-lint
helm-lint: ## Run helm lint on the chart.
	helm lint $(HELM_CHART)

.PHONY: helm-template
helm-template: ## Render chart with default + critical combinations sanity check.
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system >/dev/null && \
		echo "✓ helm template (default values) OK"
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system \
		--set features.cluster.enabled=true \
		--set features.backup.enabled=true \
		--set features.autoscaling.enabled=true >/dev/null && \
		echo "✓ helm template (all features enabled) OK"
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system \
		--set tracing.endpoint=otel-collector.observability.svc:4317 \
		--set tracing.serviceName=valkey-operator >/dev/null && \
		echo "✓ helm template (OTEL tracing enabled) OK"
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system \
		--set logging.format=console \
		--set logging.level=debug \
		--set logging.development=true >/dev/null && \
		echo "✓ helm template (debug logging) OK"
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system \
		--set webhook.enabled=true \
		--set networkPolicy.enabled=true >/dev/null && \
		echo "✓ helm template (webhook + NetworkPolicy enabled, cycles 72/73) OK"
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system \
		--set features.cluster.enabled=true \
		--set features.backup.enabled=true \
		--set webhook.enabled=true \
		--set networkPolicy.enabled=true \
		--set tracing.endpoint=otel-collector.observability.svc:4317 \
		--set 'watch.namespaces={valkey-prod}' >/dev/null && \
		echo "✓ helm template (full production stack, cycles 86/88/89 cross-feature) OK"

.PHONY: audit
audit: ## govulncheck + gosec + trivy fs — RFC 0002 L3 security 게이트.
	@echo "=== govulncheck (call-graph CVE) ==="
	@command -v $(LOCALBIN)/govulncheck >/dev/null 2>&1 || GOBIN=$(LOCALBIN) go install golang.org/x/vuln/cmd/govulncheck@latest
	$(LOCALBIN)/govulncheck ./...
	@echo "=== gosec (HIGH only) ==="
	@command -v $(LOCALBIN)/gosec >/dev/null 2>&1 || GOBIN=$(LOCALBIN) go install github.com/securego/gosec/v2/cmd/gosec@latest
	$(LOCALBIN)/gosec -quiet -severity high ./internal/... || true
	@echo "=== trivy fs (lockfile + base CVE) ==="
	@command -v trivy >/dev/null 2>&1 || { echo "[error] trivy not installed: brew install trivy"; exit 1; }
	trivy fs --severity HIGH,CRITICAL --exit-code 1 --ignore-unfixed --skip-dirs vendor,bin,tmp .

.PHONY: gate
gate: lint test helm-lint helm-template audit ## RFC 0002 L3 종합 게이트 (lint+test+helm+audit).
	@echo "✓ all gates PASS"

.PHONY: setup-hooks
setup-hooks: ## RFC 0002 L1+L2 로컬 hook 설치 (pre-commit + pre-push).
	@command -v pre-commit >/dev/null 2>&1 || { echo "pre-commit not installed: brew install pre-commit"; exit 1; }
	pre-commit install --hook-type pre-commit --hook-type pre-push
	@echo "✓ pre-commit + pre-push hooks installed"

.PHONY: release-preflight
release-preflight: require-version ## 릴리스 사전 검증 — git clean + Chart.yaml 버전 일치.
	@echo "=== Step 0/6- 릴리스 preflight (git clean + Chart 버전 일치) ==="
	@git diff --quiet || { echo "ERROR- working tree dirty"; git status --short; exit 1; }
	@git diff --cached --quiet || { echo "ERROR- staged but uncommitted changes"; exit 1; }
	@CHART_VER=$$(awk '/^version:/ { print $$2; exit }' $(HELM_CHART)/Chart.yaml); \
	TARGET_VER=$$(echo "$(VERSION)" | sed 's/^v//'); \
	if [ "$$CHART_VER" != "$$TARGET_VER" ]; then \
		echo "ERROR- $(HELM_CHART)/Chart.yaml version=$$CHART_VER, but release VERSION=$$TARGET_VER"; \
		echo "  먼저 $(HELM_CHART)/Chart.yaml 의 version + appVersion 갱신"; \
		exit 1; \
	fi
	@echo "✓ Chart.yaml version=$$(echo $(VERSION) | sed 's/^v//') 일치"

.PHONY: release
release: require-version ## 전체 로컬 릴리스 파이프라인. VERSION=vX.Y.Z 필수.
	@echo "=== Step 1/6- 로컬 게이트 (lint/test/helm/audit) ==="
	$(MAKE) gate
	@echo ""
	$(MAKE) release-preflight VERSION="$(VERSION)"
	@echo ""
	@echo "=== Step 2/6- Docker image build + push (linux/amd64-only, default builder — CLAUDE.md §2) ==="
	@TARGET_VER=$$(echo "$(VERSION)" | sed 's/^v//'); \
	COMMIT_VAL="$$(git rev-parse --short HEAD 2>/dev/null || echo none)"; \
	DATE_VAL="$$(date -u +%Y-%m-%d)"; \
	docker --context=default buildx build --platform linux/amd64 \
		--build-arg VERSION="$(VERSION)" \
		--build-arg COMMIT="$$COMMIT_VAL" \
		--build-arg BUILD_DATE="$$DATE_VAL" \
		-t "$(IMAGE_REPOSITORY):$(VERSION)" \
		-t "$(IMAGE_REPOSITORY):$$TARGET_VER" \
		--push .
	@echo ""
	@echo "=== Step 3/6- Git tag + push ==="
	@if git tag -l "$(VERSION)" | grep -q .; then \
		echo "WARN- tag $(VERSION) 이미 존재 — skip"; \
	else \
		git tag -a "$(VERSION)" -m "$(VERSION)"; \
	fi
	git push origin "$(VERSION)"
	@echo ""
	@echo "=== Step 4/6- GitHub Release (prerelease if -alpha/-beta/-rc) ==="
	@PREFLAG=""; case "$(VERSION)" in *alpha*|*beta*|*rc*) PREFLAG="--prerelease";; esac; \
	mkdir -p "$(RELEASE_TMP)"; \
	if [ "$(HELM_SIGN)" = "1" ]; then \
		$(MAKE) helm-signing-preflight VERSION="$(VERSION)" >/dev/null; \
		helm package --sign --key "$(HELM_GPG_KEY)" --keyring "$(HELM_KEYRING)" "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
	else \
		helm package "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
	fi; \
	PROV_ASSET="$$(ls "$(RELEASE_TMP)"/valkey-operator-$$(echo "$(VERSION)" | sed 's/^v//').tgz.prov 2>/dev/null || true)"; \
	if command -v git-cliff >/dev/null 2>&1; then \
		git-cliff --strip all --tag "$(VERSION)" --unreleased > "/tmp/release-notes-$(VERSION).md" 2>/dev/null && \
			NOTES_FLAG="--notes-file /tmp/release-notes-$(VERSION).md"; \
	else \
		NOTES_FLAG="--notes \"Release $(VERSION). 변경 내역은 CHANGELOG.md 참조.\""; \
	fi; \
	SBOM_ASSET=""; \
	if command -v syft >/dev/null 2>&1; then \
		echo "=== syft SBOM 생성 ==="; \
		syft scan ghcr.io/keiailab/valkey-operator:$(VERSION) -o spdx-json -q > "/tmp/valkey-operator-$(VERSION).spdx.json" 2>/dev/null && \
			SBOM_ASSET="/tmp/valkey-operator-$(VERSION).spdx.json"; \
	fi; \
	if gh release view "$(VERSION)" -R keiailab/valkey-operator >/dev/null 2>&1; then \
		echo "WARN- GH release $(VERSION) 이미 존재 — skip"; \
	else \
		eval gh release create "$(VERSION)" -R keiailab/valkey-operator $$PREFLAG \
			--title "$(VERSION)" \
			$$NOTES_FLAG \
			"$(RELEASE_TMP)/valkey-operator-$$(echo "$(VERSION)" | sed 's/^v//').tgz" \
			$$PROV_ASSET \
			$$SBOM_ASSET; \
	fi
	@rm -rf "$(RELEASE_TMP)"
	@echo ""
	@echo "=== Step 5/6- Helm chart publish to gh-pages ==="
	$(MAKE) helm-publish
	@echo ""
	@echo "=== Step 6/6- 완료 ==="
	@echo "✓ Release $(VERSION) 완료"
	@echo "  - Image: $(IMAGE_REPOSITORY):$(VERSION)"
	@echo "  - GH Release: https://github.com/keiailab/valkey-operator/releases/tag/$(VERSION)"
	@echo "  - Helm Repo: helm repo update && helm search repo keiailab/valkey-operator"
	@echo "  - ArtifactHub 은 ~30분 내 인덱스 자동 갱신"

# PGP signing 옵션 — 기본값 1. Artifact Hub Signed badge 는 Helm provenance
# 파일(.tgz.prov)이 차트와 같은 경로에 있어야 활성화된다.
# HELM_GPG_KEY 가 GnuPG keyring 에 import 되어 있어야 함 (private key, 비공개).
# Helm --key 는 fingerprint 가 아니라 UID substring 을 요구한다.
HELM_SIGN     ?= 1
HELM_GPG_KEY  ?= Keiailab Helm
HELM_GPG_FINGERPRINT ?= 89A409476828CB992338C378651E51AF520BCB78
HELM_KEYRING  ?= $(HOME)/.gnupg/secring.gpg

.PHONY: helm-signing-preflight
helm-signing-preflight: ## Helm chart provenance 서명 전제조건 검사 — Artifact Hub Signed badge 게이트.
	@if [ "$(HELM_SIGN)" != "1" ]; then \
		echo "ERROR: Artifact Hub Signed badge 를 위해 HELM_SIGN=1 이 필요"; \
		exit 1; \
	fi
	@if [ ! -s "$(HELM_KEYRING)" ]; then \
		echo "ERROR: HELM_KEYRING 누락: $(HELM_KEYRING)"; \
		echo "  GPG secret key 를 import 한 뒤: gpg --export-secret-keys > $(HELM_KEYRING)"; \
		exit 1; \
	fi
	@echo "✓ Helm signing preflight: key=$(HELM_GPG_KEY), fingerprint=$(HELM_GPG_FINGERPRINT), keyring=$(HELM_KEYRING)"

.PHONY: release-notes
release-notes: ## git-cliff 로 release notes 자동 생성 — 출력 파일 /tmp/release-notes-$(VERSION).md.
	@command -v git-cliff >/dev/null 2>&1 || { echo "[error] git-cliff not installed: brew install git-cliff"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	git-cliff --strip all --tag "$(VERSION)" --unreleased > "/tmp/release-notes-$(VERSION).md"
	@echo "✓ release notes: /tmp/release-notes-$(VERSION).md"

.PHONY: bundle
bundle: ## OperatorHub.io bundle 생성 — operator-sdk + kustomize. VERSION 필수 (e.g. 1.0.9). PR-B9 / ADR-0037 후속.
	@command -v operator-sdk >/dev/null 2>&1 || { echo "[error] operator-sdk not installed: brew install operator-sdk"; exit 1; }
	@command -v kustomize >/dev/null 2>&1 || { echo "[error] kustomize not installed"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수 (e.g. make bundle VERSION=1.0.9)"; exit 1; fi
	@echo "=== set image controller=ghcr.io/keiailab/valkey-operator:v$(VERSION) ==="
	cd config/manager && kustomize edit set image controller=ghcr.io/keiailab/valkey-operator:v$(VERSION)
	@echo "=== kustomize build config/manifests | operator-sdk generate bundle ==="
	kustomize build config/manifests | operator-sdk generate bundle \
		--overwrite \
		--version "$(VERSION)" \
		--channels alpha \
		--default-channel alpha \
		--package valkey-operator
	@echo "=== operator-sdk bundle validate ==="
	operator-sdk bundle validate ./bundle
	@echo "✓ bundle: ./bundle/ ($(VERSION), channel alpha)"

.PHONY: bundle-build
bundle-build: bundle ## bundle image 빌드 — registry push 는 별 단계 (community-operators PR 시).
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	docker buildx build --platform linux/amd64 -f bundle.Dockerfile -t ghcr.io/keiailab/valkey-operator-bundle:v$(VERSION) .
	@echo "✓ bundle image: ghcr.io/keiailab/valkey-operator-bundle:v$(VERSION)"

.PHONY: sbom
sbom: ## syft 로 SBOM (SPDX-2.3) 생성 — image 의 binary + Go modules. SLSA / EU CRA 표준.
	@command -v syft >/dev/null 2>&1 || { echo "[error] syft not installed: brew install syft"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	@echo "=== syft scan ghcr.io/keiailab/valkey-operator:$(VERSION) ==="
	syft scan ghcr.io/keiailab/valkey-operator:$(VERSION) -o spdx-json -q > "/tmp/valkey-operator-$(VERSION).spdx.json"
	@SIZE=$$(wc -c < "/tmp/valkey-operator-$(VERSION).spdx.json" | tr -d ' '); \
	echo "✓ SBOM: /tmp/valkey-operator-$(VERSION).spdx.json ($$SIZE bytes)"

.PHONY: sign-image
sign-image: ## cosign 으로 image 서명 (keyfile + Sigstore Rekor public ledger). VERSION + COSIGN_KEY 필수. ADR-0033.
	@command -v cosign >/dev/null 2>&1 || { echo "[error] cosign not installed: brew install cosign"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	@if [ -z "$(COSIGN_KEY)" ]; then echo "ERROR: COSIGN_KEY 필수 (cosign.key 경로 — keyless OIDC 는 RFC-0002 GHA 금지로 회피)"; exit 1; fi
	@echo "=== cosign sign ghcr.io/keiailab/valkey-operator:$(VERSION) ==="
	cosign sign --key "$(COSIGN_KEY)" --yes ghcr.io/keiailab/valkey-operator:$(VERSION)
	@echo "✓ image signed (Sigstore Rekor entry 생성)"

.PHONY: attest-provenance
attest-provenance: ## SLSA L2 in-toto provenance attestation (cosign attest). VERSION + COSIGN_KEY 필수. ADR-0033.
	@command -v cosign >/dev/null 2>&1 || { echo "[error] cosign not installed: brew install cosign"; exit 1; }
	@command -v jq >/dev/null 2>&1 || { echo "[error] jq not installed: brew install jq"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	@if [ -z "$(COSIGN_KEY)" ]; then echo "ERROR: COSIGN_KEY 필수"; exit 1; fi
	@echo "=== generate SLSA L2 provenance v1 statement ==="
	@PROV="/tmp/valkey-operator-$(VERSION).provenance.json"; \
	  GIT_COMMIT="$$(git rev-parse HEAD)"; \
	  jq -n --arg v "$(VERSION)" --arg img "ghcr.io/keiailab/valkey-operator:$(VERSION)" \
	    --arg now "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg sha "$$GIT_COMMIT" \
	    '{_type:"https://in-toto.io/Statement/v1", subject:[{name:$$img, digest:{}}], predicateType:"https://slsa.dev/provenance/v1", predicate:{buildDefinition:{buildType:"https://keiailab.io/valkey-operator/release/v1", externalParameters:{version:$$v, gitCommit:$$sha}}, runDetails:{builder:{id:"https://keiailab.io/valkey-operator/scripts/release.sh"}, metadata:{invocationId:$$sha, startedOn:$$now, finishedOn:$$now}}}}' \
	    > "$$PROV"; \
	  echo "=== cosign attest --predicate $$PROV --type slsaprovenance ==="; \
	  cosign attest --predicate "$$PROV" --type slsaprovenance --key "$(COSIGN_KEY)" --yes ghcr.io/keiailab/valkey-operator:$(VERSION); \
	  echo "✓ provenance attested (Rekor entry 생성)"

.PHONY: helm-docs
helm-docs: ## helm-docs 로 chart README 의 values 표 자동 생성 (values.yaml `--` 주석 → MD).
	@command -v helm-docs >/dev/null 2>&1 || { echo "[error] helm-docs not installed: brew install norwoodj/tap/helm-docs"; exit 1; }
	helm-docs --chart-search-root "$(HELM_CHART)" --template-files=README.md.gotmpl 2>/dev/null || helm-docs --chart-search-root "$(HELM_CHART)"
	@echo "✓ chart README values 표 자동 갱신"

.PHONY: helm-publish
helm-publish: ## Publish helm chart to gh-pages (RFC 0002 — GH Actions 대체 로컬 자동화). gh-pages 부재 시 auto-orphan. 기본 PGP .prov 동반.
	@echo "=== helm package ==="
	@rm -rf "$(RELEASE_TMP)" "$(GHPAGES_TMP)"
	@mkdir -p "$(RELEASE_TMP)"
	@if [ "$(HELM_SIGN)" = "1" ]; then \
		$(MAKE) helm-signing-preflight VERSION="$(VERSION)" >/dev/null; \
		echo "INFO- chart 서명 활성 (PGP key $(HELM_GPG_KEY))"; \
		helm package --sign --key "$(HELM_GPG_KEY)" --keyring "$(HELM_KEYRING)" "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
	else \
		echo "WARN- HELM_SIGN=0: Artifact Hub Signed badge 비활성 릴리스"; \
		helm package "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
	fi
	@if [ "$(HELM_SIGN)" = "1" ]; then \
		ls "$(RELEASE_TMP)"/valkey-operator-*.tgz.prov >/dev/null 2>&1 || { \
			echo "ERROR: signed chart provenance(.tgz.prov) 생성 실패"; \
			exit 1; \
		}; \
	fi
	@echo "=== gh-pages worktree (auto-orphan if branch missing) ==="
	@if git ls-remote --exit-code --heads origin gh-pages >/dev/null 2>&1; then \
		git clone --branch gh-pages --single-branch "$$(git remote get-url origin)" "$(GHPAGES_TMP)"; \
	else \
		echo "INFO- gh-pages 브랜치 부재 — orphan 으로 신규 생성"; \
		git clone "$$(git remote get-url origin)" "$(GHPAGES_TMP)"; \
		cd "$(GHPAGES_TMP)" && git checkout --orphan gh-pages && git rm -rf . >/dev/null 2>&1 || true; \
	fi
	@echo "=== copy chart + regen index ==="
	cp "$(RELEASE_TMP)"/valkey-operator-*.tgz "$(GHPAGES_TMP)/"
	@if [ "$(HELM_SIGN)" = "1" ]; then \
		cp "$(RELEASE_TMP)"/valkey-operator-*.tgz.prov "$(GHPAGES_TMP)/"; \
	else \
		cp "$(RELEASE_TMP)"/valkey-operator-*.tgz.prov "$(GHPAGES_TMP)/" 2>/dev/null || true; \
	fi
	cp "$(HELM_CHART)/../artifacthub-repo.yml" "$(GHPAGES_TMP)/" 2>/dev/null || true
	@if [ -f "$(GHPAGES_TMP)/index.yaml" ]; then \
		cd "$(GHPAGES_TMP)" && helm repo index . --merge index.yaml --url "$(HELM_REPO_URL)"; \
	else \
		cd "$(GHPAGES_TMP)" && helm repo index . --url "$(HELM_REPO_URL)"; \
	fi
	@echo "=== commit + push ==="
	@cd "$(GHPAGES_TMP)" && git add -A && \
		(git diff --cached --quiet || git commit -m "chore(helm): publish $$(awk '/^version:/ { print $$2; exit }' "$(CURDIR)/$(HELM_CHART)/Chart.yaml")") && \
		git push -u origin gh-pages
	@rm -rf "$(RELEASE_TMP)" "$(GHPAGES_TMP)"
	@echo "✓ Helm chart 게시 완료. ArtifactHub 은 ~30분 내 인덱싱."

##@ Validation (RFC-0017 §3.3 표준)

.PHONY: validate
validate: manifests ## kustomize build + helm lint + helm template — manifest 회귀 검증
	@echo "=== kustomize build config/default ==="
	@command -v kustomize >/dev/null 2>&1 || { echo "[error] kustomize 미설치 (make kustomize)"; exit 1; }
	@$(KUSTOMIZE) build config/default >/dev/null
	@echo "✓ kustomize build PASS"
	@echo "=== helm lint charts/valkey-operator ==="
	@command -v helm >/dev/null 2>&1 || { echo "[warn] helm 미설치 — skip"; exit 0; }
	@helm lint charts/valkey-operator
	@echo "✓ helm lint PASS"
	@echo "=== helm template (default values) ==="
	@helm template valkey-operator charts/valkey-operator >/dev/null
	@echo "=== helm template (모든 features on) ==="
	@helm template valkey-operator charts/valkey-operator \
		--set features.cluster.enabled=true \
		--set features.backup.enabled=true \
		--set features.autoscaling.enabled=true >/dev/null
	@echo "✓ helm template PASS"
	@echo "✓ make validate 통과 (RFC-0017 §3.3)"
