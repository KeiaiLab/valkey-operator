# NetworkPolicy CNI Enforcement 검증

valkey-operator 가 생성하는 NetworkPolicy 가 *실제로 cross-pod ingress 를
차단* 하는지 CNI enforcement 환경에서 검증.

## 배경

NetworkPolicy 는 *Kubernetes API resource* 일 뿐, 실제 packet filtering 은
*CNI plugin* (Calico / Cilium / kube-router 등) 의 enforcement 기능에 의존.
기본 CNI (kindnet, flannel) 는 NP 무시.

ValkeyController/ValkeyClusterController 는 `Spec.NetworkPolicy.Enabled=true`
시 NetworkPolicy 리소스 생성 — 본 디렉토리의 매니페스트로 enforcement 검증.

## 시나리오

### 1. Calico 환경 (kind)

```sh
# Calico 활성화 kind cluster.
cat > /tmp/kind-calico.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
  podSubnet: 192.168.0.0/16
EOF
kind create cluster --name vk-calico --config /tmp/kind-calico.yaml
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/calico.yaml
kubectl wait --for=condition=Ready -n kube-system pod -l k8s-app=calico-node --timeout=300s

# operator + Valkey 배포
make deploy IMG=valkey-operator:dev
kubectl apply -f config/samples/cache_v1alpha1_valkey.yaml
# Spec.NetworkPolicy.Enabled=true 명시 (또는 기본 enabled 일 시)

# probe pod 생성 (NP 영향 받는 같은 namespace).
kubectl run probe-allowed --image=alpine --command -- sleep 3600
kubectl run probe-blocked --image=alpine --labels="app=stranger" --command -- sleep 3600

# 검증 — same selector pod 는 6379 도달, label 다른 pod 는 차단.
kubectl exec probe-allowed -- nc -zv valkey-sample-0.valkey-sample-headless 6379  # OK
kubectl exec probe-blocked -- nc -zv valkey-sample-0.valkey-sample-headless 6379  # blocked (timeout)
```

### 2. Cilium 환경 (kind)

```sh
cat > /tmp/kind-cilium.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
EOF
kind create cluster --name vk-cilium --config /tmp/kind-cilium.yaml
helm repo add cilium https://helm.cilium.io
helm install cilium cilium/cilium --namespace kube-system \
  --set kubeProxyReplacement=strict \
  --set k8sServiceHost=$(docker inspect vk-cilium-control-plane -f '{{range $k,$v := .NetworkSettings.Networks}}{{$v.IPAddress}}{{end}}') \
  --set k8sServicePort=6443
kubectl wait --for=condition=Ready -n kube-system pod -l k8s-app=cilium --timeout=300s

# 위 Calico 시나리오 와 동일 probe 패턴.
```

### 3. 자동 검증 (별개 cycle)

`test/network-policy/np_test.go` (별개 commit) — 위 시나리오를 e2e 자동화.
현재는 *수동 검증 매니페스트* 만 보존.

## 알려진 한계

- kindnet / flannel 등 NP 미지원 CNI 환경에서 본 매니페스트 검증 불가능.
- Cilium 의 `CiliumNetworkPolicy` (extended) 와 별도 — Kubernetes 표준 NP 만
  검증 대상.

## Refs

- README L105: 알려진 한계 (NetworkPolicy 강제 동작 검증 부재)
- Plan §3 Track E: Production hardening (NP enforcement 검증 항목)
- HANDOFF.md cycle 3 §2.1
