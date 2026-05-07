/*
Copyright 2026 Keiailab.
*/

package resources

import (
	networkingv1 "k8s.io/api/networking/v1"

	commonsnp "github.com/keiailab/operator-commons/pkg/networkpolicy"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// BuildNetworkPolicy — deny-by-default + 같은 인스턴스 pod 간 6379(/16379) 허용 +
// AdditionalIngressFrom peers 허용.
//
// iteration 25 (2026-05-07): operator-commons/pkg/networkpolicy v0.3.0 위임.
// 이전 인라인 패턴 (한 rule 에 self+extra peers 합침) → commons 의 별-rule 패턴
// (WithSelfIngress + WithIngressFromPeers). K8s NetworkPolicy OR 규약상 동작 동등.
func BuildNetworkPolicy(crName, namespace string, clusterMode bool, spec *cachev1alpha1.NetworkPolicySpec) *networkingv1.NetworkPolicy {
	tcpPorts := []int32{PortClient}
	if clusterMode {
		tcpPorts = append(tcpPorts, PortClusterBus)
	}

	opts := []commonsnp.Option{
		commonsnp.WithLabels(CommonLabels(crName, "valkey")),
		commonsnp.WithSelfIngress(tcpPorts),
	}

	if spec != nil && len(spec.AdditionalIngressFrom) > 0 {
		extraPeers := make([]commonsnp.Peer, 0, len(spec.AdditionalIngressFrom))
		for _, p := range spec.AdditionalIngressFrom {
			peer := commonsnp.Peer{}
			if p.PodSelector != nil {
				peer.PodSelector = *p.PodSelector
			}
			if p.NamespaceSelector != nil {
				peer.NamespaceSelector = *p.NamespaceSelector
			}
			extraPeers = append(extraPeers, peer)
		}
		opts = append(opts, commonsnp.WithIngressFromPeers(extraPeers, tcpPorts))
	}

	return commonsnp.New(NetworkPolicyName(crName), namespace, SelectorLabels(crName), opts...)
}
