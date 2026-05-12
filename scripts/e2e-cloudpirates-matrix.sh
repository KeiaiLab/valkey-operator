#!/usr/bin/env bash
# CloudPirates chart 호환 필드 운영 승인용 E2E matrix.
#
# 실제 Kubernetes cluster 에 전용 namespace + 전용 Helm release 를 만들고,
# 운영 release 와 분리된 WATCH_NAMESPACES 로 신규 호환 필드를 검증한다.

set -Eeuo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHART_DIR="${REPO_DIR}/charts/valkey-operator"

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)}"
E2E_NAMESPACE="${E2E_NAMESPACE:-valkey-cloudpirates-e2e-${RUN_ID}}"
E2E_RELEASE="${E2E_RELEASE:-valkey-operator-e2e-${RUN_ID}}"
VALKEY_IMAGE_REF="${VALKEY_IMAGE_REF:-docker.io/valkey/valkey:9.0.4}"
OPERATOR_IMAGE_TAG="${OPERATOR_IMAGE_TAG:-}"
STORAGE_CLASS="${STORAGE_CLASS:-}"
KEEP_E2E_NAMESPACE="${KEEP_E2E_NAMESPACE:-false}"

PASS_COUNT=0
FAIL_COUNT=0
TMP_DIR=""

log() { printf '\n==> %s\n' "$*"; }
evidence() { printf '    %s\n' "$*"; }
pass() { PASS_COUNT=$((PASS_COUNT + 1)); printf '  PASS %s\n' "$*"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); printf '  FAIL %s\n' "$*" >&2; return 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "필수 명령이 없습니다: $1"
}

json() {
  local resource="$1"
  local expr="$2"
  kubectl -n "${E2E_NAMESPACE}" get "${resource}" -o json | jq -er "${expr}"
}

assert_eq() {
  local label="$1"
  local actual="$2"
  local expected="$3"
  if [[ "${actual}" == "${expected}" ]]; then
    pass "${label}: ${actual}"
  else
    fail "${label}: got=${actual} want=${expected}"
  fi
}

assert_nonempty() {
  local label="$1"
  local actual="$2"
  if [[ -n "${actual}" && "${actual}" != "null" ]]; then
    pass "${label}: ${actual}"
  else
    fail "${label}: empty"
  fi
}

retry() {
  local timeout="$1"
  local sleep_s="$2"
  shift 2
  local started now
  started="$(date +%s)"
  while true; do
    if "$@"; then
      return 0
    fi
    now="$(date +%s)"
    if (( now - started >= timeout )); then
      return 1
    fi
    sleep "${sleep_s}"
  done
}

cleanup() {
  local rc=$?
  set +e
  if [[ "${KEEP_E2E_NAMESPACE}" == "true" ]]; then
    log "KEEP_E2E_NAMESPACE=true 이므로 정리하지 않습니다: ${E2E_NAMESPACE}"
    exit "${rc}"
  fi

  log "E2E 리소스 정리"
  kubectl -n "${E2E_NAMESPACE}" delete valkey --all --ignore-not-found --timeout=180s >/dev/null 2>&1 || true
  kubectl -n "${E2E_NAMESPACE}" delete valkeycluster --all --ignore-not-found --timeout=180s >/dev/null 2>&1 || true
  helm uninstall "${E2E_RELEASE}" -n "${E2E_NAMESPACE}" >/dev/null 2>&1 || true
  kubectl delete namespace "${E2E_NAMESPACE}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  [[ -n "${TMP_DIR}" ]] && rm -rf "${TMP_DIR}"
  exit "${rc}"
}
trap cleanup EXIT

operator_tag() {
  if [[ -n "${OPERATOR_IMAGE_TAG}" ]]; then
    printf '%s\n' "${OPERATOR_IMAGE_TAG}"
    return
  fi
  awk '/^appVersion:/ { gsub(/"/, "", $2); print $2; exit }' "${CHART_DIR}/Chart.yaml"
}

default_storage_class() {
  kubectl get storageclass -o json | jq -r '
    .items[]
    | select(.metadata.annotations["storageclass.kubernetes.io/is-default-class"] == "true"
        or .metadata.annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true")
    | .metadata.name
  ' | head -n 1
}

apply_yaml() {
  kubectl apply -f -
}

wait_valkey() {
  local name="$1"
  local ready="$2"
  kubectl -n "${E2E_NAMESPACE}" wait \
    --for=jsonpath='{.status.phase}'=Running \
    "valkey/${name}" --timeout=8m
  kubectl -n "${E2E_NAMESPACE}" wait \
    --for=condition=ready \
    "pod/${name}-0" --timeout=5m
  local got
  got="$(kubectl -n "${E2E_NAMESPACE}" get valkey "${name}" -o jsonpath='{.status.readyReplicas}')"
  assert_eq "Valkey/${name} readyReplicas" "${got}" "${ready}"
}

wait_cluster() {
  local name="$1"
  local ready="$2"
  kubectl -n "${E2E_NAMESPACE}" wait \
    --for=jsonpath='{.status.phase}'=Running \
    "valkeycluster/${name}" --timeout=12m
  retry 300 5 bash -c \
    "test \"\$(kubectl -n '${E2E_NAMESPACE}' get valkeycluster '${name}' -o jsonpath='{.status.clusterState}/{.status.assignedSlots}/{.status.readyReplicas}')\" = 'ok/16384/${ready}'"
  pass "ValkeyCluster/${name}: clusterState=ok assignedSlots=16384 readyReplicas=${ready}"
}

exec_valkey() {
  local pod="$1"
  shift
  kubectl -n "${E2E_NAMESPACE}" exec "${pod}" -- sh -c "$*"
}

preflight() {
  log "사전 조건 확인"
  require_cmd kubectl
  require_cmd helm
  require_cmd jq
  require_cmd openssl

  kubectl version --client=true >/dev/null
  helm version --short >/dev/null

  local ctx tag
  ctx="$(kubectl config current-context)"
  tag="$(operator_tag)"
  assert_nonempty "kubectl context" "${ctx}"
  assert_nonempty "operator image tag" "${tag}"

  if [[ -z "${STORAGE_CLASS}" ]]; then
    STORAGE_CLASS="$(default_storage_class)"
  fi
  assert_nonempty "storageClass" "${STORAGE_CLASS}"

  evidence "namespace=${E2E_NAMESPACE}"
  evidence "release=${E2E_RELEASE}"
  evidence "operator=ghcr.io/keiailab/valkey-operator:${tag}"
  evidence "valkey=${VALKEY_IMAGE_REF}"
}

install_operator() {
  local tag
  tag="$(operator_tag)"

  log "격리 namespace 와 operator 설치"
  kubectl create namespace "${E2E_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
  kubectl label namespace "${E2E_NAMESPACE}" \
    "pod-security.kubernetes.io/enforce=restricted" \
    "e2e.keiailab.io/run=${RUN_ID}" \
    --overwrite >/dev/null

  kubectl apply -f "${REPO_DIR}/config/crd/bases" >/dev/null

  helm upgrade --install "${E2E_RELEASE}" "${CHART_DIR}" \
    --namespace "${E2E_NAMESPACE}" \
    --skip-crds \
    --wait \
    --timeout 5m \
    --set image.tag="${tag}" \
    --set image.pullPolicy=IfNotPresent \
    --set features.cluster.enabled=true \
    --set features.autoscaling.enabled=true \
    --set features.backup.enabled=false \
    --set metrics.serviceMonitor.enabled=false \
    --set "watch.namespaces[0]=${E2E_NAMESPACE}" >/dev/null

  local deploy
  deploy="$(kubectl -n "${E2E_NAMESPACE}" get deploy \
    -l "app.kubernetes.io/instance=${E2E_RELEASE}" \
    -o jsonpath='{.items[0].metadata.name}')"
  kubectl -n "${E2E_NAMESPACE}" rollout status "deploy/${deploy}" --timeout=5m
  assert_eq "operator WATCH_NAMESPACES" \
    "$(json "deploy/${deploy}" '.spec.template.spec.containers[0].env[] | select(.name == "WATCH_NAMESPACES").value')" \
    "${E2E_NAMESPACE}"
}

create_tls_secret() {
  log "TLS customCert secret 생성"
  TMP_DIR="$(mktemp -d)"
  openssl req -x509 -nodes -newkey rsa:2048 -days 2 \
    -keyout "${TMP_DIR}/tls.key" \
    -out "${TMP_DIR}/tls.crt" \
    -subj "/CN=cp-tls" \
    -addext "subjectAltName=DNS:cp-tls,DNS:cp-tls.${E2E_NAMESPACE}.svc,DNS:cp-tls-headless.${E2E_NAMESPACE}.svc,DNS:*.cp-tls-headless.${E2E_NAMESPACE}.svc" \
    >/dev/null 2>&1
  cp "${TMP_DIR}/tls.crt" "${TMP_DIR}/ca.crt"
  kubectl -n "${E2E_NAMESPACE}" create secret generic cp-tls-custom-cert \
    --from-file=tls.crt="${TMP_DIR}/tls.crt" \
    --from-file=tls.key="${TMP_DIR}/tls.key" \
    --from-file=ca.crt="${TMP_DIR}/ca.crt" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  pass "TLS Secret/cp-tls-custom-cert 생성"
}

case_persistent() {
  log "[1/6] persistent + service/pod/storage metadata + PDB/NetworkPolicy"
  cat <<EOF | apply_yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-persistent
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Standalone
  replicas: 1
  revisionHistoryLimit: 2
  version:
    imageRef: ${VALKEY_IMAGE_REF}
    imagePullPolicy: IfNotPresent
  storage:
    storageClassName: ${STORAGE_CLASS}
    size: 1Gi
    accessModes:
      - ReadWriteOnce
    labels:
      e2e.keiailab.io/storage: persistent
    annotations:
      e2e.keiailab.io/storage-note: persistent
  service:
    type: ClusterIP
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv4
    labels:
      e2e.keiailab.io/service: persistent
    annotations:
      e2e.keiailab.io/service-note: persistent
  pod:
    labels:
      e2e.keiailab.io/pod: persistent
    annotations:
      e2e.keiailab.io/pod-note: persistent
    hostAliases:
      - ip: 127.0.0.1
        hostnames:
          - cloudpirates.local
    extraEnv:
      - name: E2E_CASE
        value: persistent
    startupProbe:
      exec:
        command:
          - sh
          - -c
          - valkey-cli -h 127.0.0.1 -p 6379 \${VALKEY_PASSWORD:+-a "\$VALKEY_PASSWORD"} ping | grep -q PONG
      initialDelaySeconds: 1
      periodSeconds: 5
      failureThreshold: 30
    terminationGracePeriodSeconds: 20
  podDisruptionBudget:
    enabled: true
    minAvailable: 1
  networkPolicy:
    enabled: true
  persistence:
    mode: None
  additionalConfig:
    maxmemory-policy: allkeys-lru
EOF
  wait_valkey cp-persistent 1

  assert_eq "cp-persistent imageRef" "$(json sts/cp-persistent '.spec.template.spec.containers[] | select(.name == "valkey").image')" "${VALKEY_IMAGE_REF}"
  assert_eq "cp-persistent revisionHistoryLimit" "$(json sts/cp-persistent '.spec.revisionHistoryLimit | tostring')" "2"
  assert_eq "cp-persistent PVC accessMode" "$(json sts/cp-persistent '.spec.volumeClaimTemplates[0].spec.accessModes[0]')" "ReadWriteOnce"
  assert_eq "cp-persistent PVC label" "$(json sts/cp-persistent '.spec.volumeClaimTemplates[0].metadata.labels["e2e.keiailab.io/storage"]')" "persistent"
  assert_eq "cp-persistent pod label" "$(json sts/cp-persistent '.spec.template.metadata.labels["e2e.keiailab.io/pod"]')" "persistent"
  assert_eq "cp-persistent extraEnv" "$(json sts/cp-persistent '.spec.template.spec.containers[] | select(.name == "valkey").env[] | select(.name == "E2E_CASE").value')" "persistent"
  assert_eq "cp-persistent hostAlias" "$(json sts/cp-persistent '.spec.template.spec.hostAliases[0].hostnames[0]')" "cloudpirates.local"
  assert_eq "cp-persistent service label" "$(json svc/cp-persistent '.metadata.labels["e2e.keiailab.io/service"]')" "persistent"
  assert_eq "cp-persistent service ipFamily" "$(json svc/cp-persistent '.spec.ipFamilies[0]')" "IPv4"
  kubectl -n "${E2E_NAMESPACE}" get pdb cp-persistent >/dev/null
  pass "cp-persistent PDB 생성"
  kubectl -n "${E2E_NAMESPACE}" get networkpolicy cp-persistent >/dev/null
  pass "cp-persistent NetworkPolicy 생성"
}

case_tls_ephemeral() {
  log "[2/6] ephemeral + TLS customCert"
  cat <<EOF | apply_yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-tls
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Standalone
  replicas: 1
  revisionHistoryLimit: 1
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    ephemeral: true
  service:
    type: ClusterIP
  tls:
    enabled: true
    clientAuth: disabled
    customCert:
      secretName: cp-tls-custom-cert
EOF
  wait_valkey cp-tls 1

  assert_eq "cp-tls volumeClaimTemplates 없음" "$(json sts/cp-tls '.spec.volumeClaimTemplates | length | tostring')" "0"
  assert_eq "cp-tls emptyDir" "$(json sts/cp-tls '.spec.template.spec.volumes[] | select(.name == "data").emptyDir | type')" "object"
  assert_eq "cp-tls secret mount" "$(json sts/cp-tls '.spec.template.spec.volumes[] | select(.name == "tls").secret.secretName')" "cp-tls-custom-cert"
  assert_eq "cp-tls service TLS port" "$(json svc/cp-tls '.spec.ports[] | select(.name == "client-tls").port | tostring')" "6380"
  retry 120 5 exec_valkey cp-tls-0 'valkey-cli --tls --insecure -p 6380 ${VALKEY_PASSWORD:+-a "$VALKEY_PASSWORD"} ping | grep -q PONG'
  pass "cp-tls TLS PING 성공"
}

case_existing_claim() {
  log "[3/6] existingClaim"
  cat <<EOF | apply_yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: cp-existing-data
  namespace: ${E2E_NAMESPACE}
  labels:
    e2e.keiailab.io/existing-claim: "true"
spec:
  storageClassName: ${STORAGE_CLASS}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
---
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-existing
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Standalone
  replicas: 1
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    existingClaim: cp-existing-data
EOF
  wait_valkey cp-existing 1

  assert_eq "cp-existing volumeClaimTemplates 없음" "$(json sts/cp-existing '.spec.volumeClaimTemplates | length | tostring')" "0"
  assert_eq "cp-existing PVC 참조" "$(json sts/cp-existing '.spec.template.spec.volumes[] | select(.name == "data").persistentVolumeClaim.claimName')" "cp-existing-data"
  assert_eq "cp-existing PVC 유지" "$(json pvc/cp-existing-data '.metadata.labels["e2e.keiailab.io/existing-claim"]')" "true"
}

case_external_replica() {
  log "[4/6] externalReplica"
  cat <<EOF | apply_yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-external-upstream
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Standalone
  replicas: 1
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    ephemeral: true
---
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-external-replica
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Standalone
  replicas: 1
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    ephemeral: true
  externalReplica:
    enabled: true
    host: cp-external-upstream.${E2E_NAMESPACE}.svc
    port: 6379
    auth:
      enabled: true
      passwordSecretRef:
        name: cp-external-upstream-auth
        key: password
EOF
  wait_valkey cp-external-upstream 1
  exec_valkey cp-external-upstream-0 'valkey-cli ${VALKEY_PASSWORD:+-a "$VALKEY_PASSWORD"} set cloudpirates:e2e replicated | grep -q OK'
  wait_valkey cp-external-replica 1

  local replica_conf
  replica_conf="$(json configmap/cp-external-replica-config '.data["valkey.conf"]')"
  if grep -Fq "replicaof cp-external-upstream.${E2E_NAMESPACE}.svc 6379" <<<"${replica_conf}"; then
    pass "cp-external-replica config replicaof 반영"
  else
    fail "cp-external-replica config replicaof 누락"
  fi
  if grep -Fq "masterauth" <<<"${replica_conf}"; then
    pass "cp-external-replica config external masterauth 반영"
  else
    fail "cp-external-replica config external masterauth 누락"
  fi
  retry 180 5 exec_valkey cp-external-replica-0 'valkey-cli ${VALKEY_PASSWORD:+-a "$VALKEY_PASSWORD"} get cloudpirates:e2e | grep -q replicated'
  pass "cp-external-replica 데이터 복제 확인"
}

case_autoscaling() {
  log "[5/6] replication autoscaling"
  cat <<EOF | apply_yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-autoscale
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Replication
  replicas: 2
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    ephemeral: true
  resources:
    requests:
      cpu: 20m
      memory: 64Mi
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 3
    targetCPUUtilizationPercentage: 60
    targetMemoryUtilizationPercentage: 80
EOF
  wait_valkey cp-autoscale 2

  assert_eq "cp-autoscale HPA minReplicas" "$(json hpa/cp-autoscale '.spec.minReplicas | tostring')" "2"
  assert_eq "cp-autoscale HPA maxReplicas" "$(json hpa/cp-autoscale '.spec.maxReplicas | tostring')" "3"
  assert_eq "cp-autoscale HPA target" "$(json hpa/cp-autoscale '.spec.scaleTargetRef.kind + "/" + .spec.scaleTargetRef.name')" "StatefulSet/cp-autoscale"
}

case_cluster_monitoring() {
  log "[6/6] ValkeyCluster + monitoring exporter"
  cat <<EOF | apply_yaml
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyCluster
metadata:
  name: cp-cluster
  namespace: ${E2E_NAMESPACE}
spec:
  shards: 3
  replicasPerShard: 0
  revisionHistoryLimit: 2
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    ephemeral: true
  service:
    type: ClusterIP
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv4
    labels:
      e2e.keiailab.io/service: cluster
  pod:
    labels:
      e2e.keiailab.io/pod: cluster
  monitoring:
    enabled: true
EOF
  wait_cluster cp-cluster 3

  assert_eq "cp-cluster revisionHistoryLimit" "$(json sts/cp-cluster '.spec.revisionHistoryLimit | tostring')" "2"
  assert_eq "cp-cluster emptyDir" "$(json sts/cp-cluster '.spec.template.spec.volumes[] | select(.name == "data").emptyDir | type')" "object"
  assert_eq "cp-cluster metrics sidecar" "$(json sts/cp-cluster '[.spec.template.spec.containers[].name] | index("metrics") | type')" "number"
  kubectl -n "${E2E_NAMESPACE}" get service cp-cluster-metrics >/dev/null
  pass "cp-cluster metrics Service 생성"
  if kubectl get crd servicemonitors.monitoring.coreos.com >/dev/null 2>&1; then
    kubectl -n "${E2E_NAMESPACE}" get servicemonitor cp-cluster >/dev/null
    pass "cp-cluster ServiceMonitor 생성"
  else
    pass "ServiceMonitor CRD 미설치 환경에서 fail-soft 동작"
  fi
  exec_valkey cp-cluster-0 'valkey-cli ${VALKEY_PASSWORD:+-a "$VALKEY_PASSWORD"} cluster info | grep -q cluster_state:ok'
  pass "cp-cluster valkey-cli cluster info OK"
}

dry_run_edges() {
  log "추가 edge manifest server dry-run"
  cat <<EOF | kubectl apply --dry-run=server -f - >/dev/null
apiVersion: cache.keiailab.io/v1alpha1
kind: Valkey
metadata:
  name: cp-dryrun-loadbalancer
  namespace: ${E2E_NAMESPACE}
spec:
  mode: Standalone
  replicas: 1
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    ephemeral: true
  pod:
    imagePullSecrets:
      - name: optional-private-registry
  service:
    type: LoadBalancer
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv4
---
apiVersion: cache.keiailab.io/v1alpha1
kind: ValkeyCluster
metadata:
  name: cp-dryrun-cluster-pvc
  namespace: ${E2E_NAMESPACE}
spec:
  shards: 3
  replicasPerShard: 1
  version:
    imageRef: ${VALKEY_IMAGE_REF}
  storage:
    storageClassName: ${STORAGE_CLASS}
    size: 1Gi
    accessModes:
      - ReadWriteOnce
    labels:
      e2e.keiailab.io/storage: dryrun
EOF
  pass "LoadBalancer/imagePullSecrets/cluster PVC edge server dry-run 통과"
}

final_evidence() {
  log "최종 evidence 수집"
  kubectl -n "${E2E_NAMESPACE}" get valkey,valkeycluster,sts,svc,hpa,pdb,networkpolicy,pod,pvc || true

  local deploy
  deploy="$(kubectl -n "${E2E_NAMESPACE}" get deploy \
    -l "app.kubernetes.io/instance=${E2E_RELEASE}" \
    -o jsonpath='{.items[0].metadata.name}')"
  if kubectl -n "${E2E_NAMESPACE}" logs "deploy/${deploy}" --since=30m --tail=2000 |
    grep -E '"level":"error"|forbidden|Failed to watch|Reconciler error' >/tmp/valkey-cloudpirates-e2e-errors.txt; then
    cat /tmp/valkey-cloudpirates-e2e-errors.txt >&2
    fail "operator error log 발견"
  else
    pass "operator 최근 로그 error/forbidden/watch 실패 없음"
  fi
}

main() {
  preflight
  install_operator
  create_tls_secret
  case_persistent
  case_tls_ephemeral
  case_existing_claim
  case_external_replica
  case_autoscaling
  case_cluster_monitoring
  dry_run_edges
  final_evidence

  log "RESULT: ${PASS_COUNT} PASS / ${FAIL_COUNT} FAIL"
  [[ "${FAIL_COUNT}" -eq 0 ]]
}

main "$@"
