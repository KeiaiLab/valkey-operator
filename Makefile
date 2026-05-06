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
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: integration-test
integration-test: fmt vet ## Run real-Valkey integration tests (requires Docker daemon). Tag: integration.
	go test -tags=integration -count=1 -timeout=180s -v ./internal/valkey/...

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
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name valkey-operator-builder
	$(CONTAINER_TOOL) buildx use valkey-operator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm valkey-operator-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml

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
helm-template: ## Render chart with default values for sanity check.
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system >/dev/null && \
		echo "✓ helm template (default values) OK"
	helm template valkey-operator $(HELM_CHART) --namespace valkey-operator-system \
		--set features.cluster.enabled=true \
		--set features.backup.enabled=true \
		--set features.autoscaling.enabled=true >/dev/null && \
		echo "✓ helm template (all features enabled) OK"

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
	@echo "=== Step 2/6- Docker image build + push (linux/amd64, default builder) ==="
	@TARGET_VER=$$(echo "$(VERSION)" | sed 's/^v//'); \
	docker --context=default buildx build --platform linux/amd64 \
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
	helm package "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
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

# PGP signing 옵션 — HELM_SIGN=1 시 helm package --sign 으로 .prov 파일 자동 생성.
# HELM_GPG_KEY 가 GnuPG keyring 에 import 되어 있어야 함 (private key, 비공개).
# 미설정 시 .prov 없이 chart 만 publish (기본 동작).
HELM_SIGN     ?= 0
HELM_GPG_KEY  ?= 89A409476828CB992338C378651E51AF520BCB78
HELM_KEYRING  ?= $(HOME)/.gnupg/secring.gpg

.PHONY: release-notes
release-notes: ## git-cliff 로 release notes 자동 생성 — 출력 파일 /tmp/release-notes-$(VERSION).md.
	@command -v git-cliff >/dev/null 2>&1 || { echo "[error] git-cliff not installed: brew install git-cliff"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	git-cliff --strip all --tag "$(VERSION)" --unreleased > "/tmp/release-notes-$(VERSION).md"
	@echo "✓ release notes: /tmp/release-notes-$(VERSION).md"

.PHONY: sbom
sbom: ## syft 로 SBOM (SPDX-2.3) 생성 — image 의 binary + Go modules. SLSA / EU CRA 표준.
	@command -v syft >/dev/null 2>&1 || { echo "[error] syft not installed: brew install syft"; exit 1; }
	@if [ -z "$(VERSION)" ]; then echo "ERROR: VERSION 필수"; exit 1; fi
	@echo "=== syft scan ghcr.io/keiailab/valkey-operator:$(VERSION) ==="
	syft scan ghcr.io/keiailab/valkey-operator:$(VERSION) -o spdx-json -q > "/tmp/valkey-operator-$(VERSION).spdx.json"
	@SIZE=$$(wc -c < "/tmp/valkey-operator-$(VERSION).spdx.json" | tr -d ' '); \
	echo "✓ SBOM: /tmp/valkey-operator-$(VERSION).spdx.json ($$SIZE bytes)"

.PHONY: helm-docs
helm-docs: ## helm-docs 로 chart README 의 values 표 자동 생성 (values.yaml `--` 주석 → MD).
	@command -v helm-docs >/dev/null 2>&1 || { echo "[error] helm-docs not installed: brew install norwoodj/tap/helm-docs"; exit 1; }
	helm-docs --chart-search-root "$(HELM_CHART)" --template-files=README.md.gotmpl 2>/dev/null || helm-docs --chart-search-root "$(HELM_CHART)"
	@echo "✓ chart README values 표 자동 갱신"

.PHONY: helm-publish
helm-publish: ## Publish helm chart to gh-pages (RFC 0002 — GH Actions 대체 로컬 자동화). gh-pages 부재 시 auto-orphan. HELM_SIGN=1 시 PGP .prov 동반.
	@echo "=== helm package ==="
	@rm -rf "$(RELEASE_TMP)" "$(GHPAGES_TMP)"
	@mkdir -p "$(RELEASE_TMP)"
	@if [ "$(HELM_SIGN)" = "1" ]; then \
		echo "INFO- chart 서명 활성 (PGP key $(HELM_GPG_KEY))"; \
		helm package --sign --key "$(HELM_GPG_KEY)" --keyring "$(HELM_KEYRING)" "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
	else \
		helm package "$(HELM_CHART)" -d "$(RELEASE_TMP)"; \
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
	@cp "$(RELEASE_TMP)"/valkey-operator-*.tgz.prov "$(GHPAGES_TMP)/" 2>/dev/null || true
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
