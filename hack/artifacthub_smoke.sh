#!/usr/bin/env bash
# shellcheck disable=SC2016
set -euo pipefail

artifacthub_api_url="${ARTIFACTHUB_API_URL:-https://artifacthub.io/api/v1}"
artifacthub_org="${ARTIFACTHUB_ORG:-keiailab}"
artifacthub_package_name="${ARTIFACTHUB_PACKAGE_NAME:-valkey-operator}"
artifacthub_repository_name="${ARTIFACTHUB_REPOSITORY_NAME:-keiailab-valkey-operator}"
helm_oci_repo="${HELM_OCI_REPO:-oci://ghcr.io/keiailab/charts}"
artifacthub_repository_url="${EXPECTED_ARTIFACTHUB_REPOSITORY_URL:-${ARTIFACTHUB_REPOSITORY_URL:-${helm_oci_repo%/}/${artifacthub_package_name}}}"
helm_repo_url="${HELM_REPO_URL:-https://keiailab.github.io/valkey-operator}"
artifacthub_api_key_id="${AH_API_KEY_ID:-${ARTIFACTHUB_API_KEY_ID:-}}"
artifacthub_api_key_secret="${AH_API_KEY_SECRET:-${ARTIFACTHUB_API_KEY_SECRET:-}}"
require_provenance="${REQUIRE_PROVENANCE:-1}"
check_container_images="${CHECK_CONTAINER_IMAGES:-1}"

curl_bin="${CURL_BIN:-curl}"
helm_bin="${HELM_BIN:-helm}"
jq_bin="${JQ_BIN:-jq}"
smoke_attempts="${ARTIFACTHUB_SMOKE_ATTEMPTS:-1}"
smoke_sleep_seconds="${ARTIFACTHUB_SMOKE_SLEEP_SECONDS:-30}"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/valkey-operator-artifacthub.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

normalize_url() {
	local url="$1"
	url="${url%/}"
	printf '%s\n' "$url"
}

require_tool() {
	local tool="$1"
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "ERROR: required tool not found: $tool" >&2
		exit 1
	fi
}

urlencode() {
	local value="$1"
	VALUE="$value" python3 - <<'PY'
import os
import urllib.parse

print(urllib.parse.quote(os.environ["VALUE"], safe=""))
PY
}

fetch_json() {
	local url="$1"
	local out="$2"
	"$curl_bin" -fsSL "$url" -o "$out"
}

echo "=== Legacy Helm repository reachability (warning-only) ==="
if "$curl_bin" -fsSL "${helm_repo_url%/}/index.yaml" -o "$tmpdir/index.yaml" 2>/dev/null; then
	echo "Legacy Helm repository reachable: ${helm_repo_url%/}"
else
	echo "::warning::legacy Helm repository is unreachable; Artifact Hub tracks OCI, so this is not a gate."
fi
if command -v "$helm_bin" >/dev/null 2>&1; then
	"$helm_bin" repo add "$artifacthub_repository_name" "$helm_repo_url" >/dev/null 2>&1 || true
	"$helm_bin" repo update "$artifacthub_repository_name" >/dev/null
	if "$helm_bin" search repo "${artifacthub_repository_name}/${artifacthub_package_name}" --versions --devel \
		| grep -q "${artifacthub_repository_name}/${artifacthub_package_name}"; then
		echo "Legacy Helm index package visible: ${artifacthub_repository_name}/${artifacthub_package_name}"
	else
		echo "::warning::legacy Helm index package not visible; Artifact Hub tracks OCI, so this is not a gate."
	fi
else
	echo "WARN: helm not found; local Helm index search skipped" >&2
fi

require_tool "$curl_bin"
require_tool "$jq_bin"

echo "=== Artifact Hub repository registration ==="
org_query="$(urlencode "$artifacthub_org")"
fetch_json "${artifacthub_api_url%/}/repositories/search?org=${org_query}&kind=0&limit=60" "$tmpdir/repositories.json"

normalized_artifacthub_repository_url="$(normalize_url "$artifacthub_repository_url")"
repo_filter='
	.[]?
	| select(.name == $name and ((.url // "" | sub("/$"; "")) == $url))
'
repo_json="$("$jq_bin" -e -c --arg url "$normalized_artifacthub_repository_url" --arg name "$artifacthub_repository_name" "$repo_filter" "$tmpdir/repositories.json" 2>/dev/null || true)"

if [[ -z "$repo_json" ]]; then
	echo "ERROR: Artifact Hub repository is not registered." >&2
	echo "  org: ${artifacthub_org}" >&2
	echo "  expected name: ${artifacthub_repository_name}" >&2
	echo "  expected url: ${normalized_artifacthub_repository_url}" >&2
	echo "  fix: make artifacthub-register ARTIFACTHUB_API_KEY_ID=... ARTIFACTHUB_API_KEY_SECRET=..." >&2
	exit 2
fi

repo_id="$("$jq_bin" -r '.repository_id' <<<"$repo_json")"
tracking_errors="$("$jq_bin" -r '.last_tracking_errors // empty' <<<"$repo_json")"
echo "Artifact Hub repository OK: ${repo_id}"

if [[ -n "$tracking_errors" ]]; then
	echo "ERROR: Artifact Hub repository tracking errors:" >&2
	echo "$tracking_errors" >&2
	exit 3
fi

if [[ -n "$artifacthub_api_key_id" && -n "$artifacthub_api_key_secret" ]]; then
	repo_display_name="$("$jq_bin" -r '.display_name // "Keiailab Valkey Operator"' <<<"$repo_json")"
	repo_update_body="$("$jq_bin" -n \
		--arg name "$artifacthub_repository_name" \
		--arg display_name "$repo_display_name" \
		--arg url "$normalized_artifacthub_repository_url" \
		'{kind: 0, name: $name, display_name: $display_name, url: $url}')"
	if "$curl_bin" -fsSL \
		-X PUT "${artifacthub_api_url%/}/repositories/org/${artifacthub_org}/${artifacthub_repository_name}" \
		-H "Content-Type: application/json" \
		-H "X-API-KEY-ID: ${artifacthub_api_key_id}" \
		-H "X-API-KEY-SECRET: ${artifacthub_api_key_secret}" \
		-d "$repo_update_body" \
		-o /dev/null; then
		echo "Artifact Hub repository update accepted (tracker nudge)"
	else
		echo "::warning::Artifact Hub repository update nudge failed; continuing with passive tracker retry."
	fi
fi

# VERSION: Chart.yaml에서 추출 (TAG 환경변수가 없을 때 fallback)
VERSION="${TAG:-}"
VERSION="${VERSION#refs/tags/}"
VERSION="${VERSION#v}"
if [[ -z "$VERSION" ]]; then
	chart_yaml="$(dirname "$0")/../charts/${artifacthub_package_name}/Chart.yaml"
	if [[ -f "$chart_yaml" ]]; then
		VERSION="$(grep '^version:' "$chart_yaml" | awk '{print $2}' | tr -d '"')"
	fi
fi

if [[ -z "$VERSION" ]]; then
	echo "ERROR: VERSION 미확인 — Chart.yaml 또는 TAG 값을 확인하세요." >&2
	exit 5
fi

echo "=== Expected release contract ==="
echo "Chart version: ${VERSION}"
echo "Helm OCI repository base: ${helm_oci_repo%/}"
echo "Artifact Hub repository URL: ${artifacthub_repository_url%/}"
echo "Smoke attempts: ${smoke_attempts} (sleep ${smoke_sleep_seconds}s)"

echo "=== GHCR OCI chart availability ==="
if command -v "$helm_bin" >/dev/null 2>&1; then
	oci_chart_ready=false
	for attempt in $(seq 1 "$smoke_attempts"); do
		if "$helm_bin" show chart "$normalized_artifacthub_repository_url" --version "$VERSION" >"$tmpdir/oci-chart.yaml" 2>"$tmpdir/oci-chart.err"; then
			oci_chart_ready=true
			break
		fi
		if [[ "$attempt" -lt "$smoke_attempts" ]]; then
			echo "GHCR OCI chart not visible yet (${attempt}/${smoke_attempts}); waiting ${smoke_sleep_seconds}s..."
			sleep "$smoke_sleep_seconds"
		fi
	done
	if [[ "$oci_chart_ready" != "true" ]]; then
		cat "$tmpdir/oci-chart.err" >&2 || true
		echo "ERROR: GHCR OCI chart is not published yet." >&2
		echo "  chart: ${normalized_artifacthub_repository_url}" >&2
		echo "  version: ${VERSION}" >&2
		exit 5
	fi
	echo "GHCR OCI chart OK: ${normalized_artifacthub_repository_url}:${VERSION}"
else
	echo "ERROR: helm not found; cannot verify GHCR OCI chart." >&2
	exit 5
fi

echo "=== Artifact Hub package registration ==="
package_url="${artifacthub_api_url%/}/packages/helm/${artifacthub_repository_name}/${artifacthub_package_name}"
package_version_url="${package_url}/${VERSION}"
artifacthub_package_ready=false
for attempt in $(seq 1 "$smoke_attempts"); do
	if "$curl_bin" -fsSL "$package_version_url" -o "$tmpdir/package.json" 2>"$tmpdir/package.err"; then
		if "$jq_bin" -e --arg name "$artifacthub_package_name" --arg version "$VERSION" \
			'.name == $name and .version == $version and .signed == true' "$tmpdir/package.json" >/dev/null; then
			artifacthub_package_ready=true
			break
		fi
		if [[ "$attempt" -lt "$smoke_attempts" ]]; then
			echo "Artifact Hub target metadata not ready yet (${attempt}/${smoke_attempts}); waiting ${smoke_sleep_seconds}s..."
			"$jq_bin" '{version, app_version, signed, repository: .repository.url}' "$tmpdir/package.json"
			sleep "$smoke_sleep_seconds"
		fi
	else
		if [[ "$attempt" -lt "$smoke_attempts" ]]; then
			echo "Artifact Hub target version not indexed yet (${attempt}/${smoke_attempts}); waiting ${smoke_sleep_seconds}s..."
			sleep "$smoke_sleep_seconds"
		fi
	fi
done
if [[ "$artifacthub_package_ready" != "true" ]]; then
	echo "ERROR: Artifact Hub repository exists but package is not indexed yet." >&2
	echo "  package API: $package_version_url" >&2
	echo "  retry after Artifact Hub tracker runs, or push a new chart version to force reprocessing." >&2
	exit 4
fi

echo "Artifact Hub package OK: https://artifacthub.io/packages/helm/${artifacthub_repository_name}/${artifacthub_package_name}?modal=version-${VERSION}"

echo "=== Helm/Artifact Hub target version 정합성 ==="
if command -v "$helm_bin" >/dev/null 2>&1; then
	if ! "$helm_bin" show chart "${artifacthub_repository_name}/${artifacthub_package_name}" --version "$VERSION" >/dev/null; then
		echo "ERROR: Helm index에 target chart version이 없습니다: ${artifacthub_repository_name}/${artifacthub_package_name}:${VERSION}" >&2
		echo "  fix: chart ${VERSION}을 gh-pages Helm index에 publish하세요." >&2
		exit 5
	fi
	echo "Helm index version OK: ${VERSION}"
fi

indexed_version="$("$jq_bin" -r '.version // empty' "$tmpdir/package.json")"
if [[ "$indexed_version" != "$VERSION" ]]; then
	echo "ERROR: Artifact Hub 최신 패키지 버전 불일치" >&2
	echo "  expected: ${VERSION}" >&2
	echo "  actual:   ${indexed_version:-<empty>}" >&2
	echo "  fix: chart ${VERSION}을 gh-pages Helm index에 publish한 뒤 Artifact Hub tracker 재동기화 대기" >&2
	exit 6
fi
echo "Artifact Hub indexed version OK: ${VERSION}"

echo "=== Artifact Hub container image 도달성 ==="
if [[ "$check_container_images" == "1" ]]; then
	if command -v docker >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1; then
		while IFS= read -r image; do
			[[ -z "$image" ]] && continue
			echo "→ image 확인: ${image}"
			if ! docker buildx imagetools inspect "$image" >/dev/null; then
				echo "ERROR: Artifact Hub 메타데이터가 존재하지 않는 컨테이너 이미지를 참조합니다: ${image}" >&2
				exit 7
			fi
		done < <("$jq_bin" -r '.containers_images[]?.image // empty' "$tmpdir/package.json")
		echo "Container images OK"
	else
		echo "WARN: docker buildx unavailable; container image reachability skipped" >&2
	fi
fi

echo "=== Provenance (.prov) 도달성 ==="

verify_provenance() {
	local prov="${helm_repo_url%/}/${artifacthub_package_name}-${VERSION}.tgz.prov"
	echo "→ provenance 확인: ${prov}"
	if "$curl_bin" -fsSL -o "$tmpdir/chart.tgz.prov" "${prov}" 2>/dev/null; then
		echo "✓ .prov 도달 가능 (Signed badge 전제 충족)"
	else
		if [[ "$require_provenance" == "1" ]]; then
			echo "ERROR: .prov 부재 — Signed badge 미달성(로컬 helm-publish.sh --sign 필요)." >&2
			exit 8
		fi
		echo "::warning::.prov 부재 — Signed badge 미달성(로컬 helm-publish.sh --sign 필요). REQUIRE_PROVENANCE=0 이므로 warn-only."
	fi
}

verify_provenance
