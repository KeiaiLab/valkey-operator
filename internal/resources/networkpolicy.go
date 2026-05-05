/*
Copyright 2026 Keiailab.
*/

package resources

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	cachev1alpha1 "github.com/keiailab/valkey-operator/api/v1alpha1"
)

// BuildNetworkPolicy — deny-by-default + 같은 인스턴스 pod 간 6379(/16379) 허용 +
// AdditionalIngressFrom peers 허용.
func BuildNetworkPolicy(crName, namespace string, clusterMode bool, spec *cachev1alpha1.NetworkPolicySpec) *networkingv1.NetworkPolicy {
	tcp := corev1.ProtocolTCP
	ports := []networkingv1.NetworkPolicyPort{
		{Protocol: &tcp, Port: portRef(PortClient)},
	}
	if clusterMode {
		ports = append(ports, networkingv1.NetworkPolicyPort{Protocol: &tcp, Port: portRef(PortClusterBus)})
	}

	selfPeer := networkingv1.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{MatchLabels: SelectorLabels(crName)},
	}

	from := []networkingv1.NetworkPolicyPeer{selfPeer}
	if spec != nil {
		for _, p := range spec.AdditionalIngressFrom {
			peer := networkingv1.NetworkPolicyPeer{}
			if p.PodSelector != nil {
				peer.PodSelector = &metav1.LabelSelector{MatchLabels: *p.PodSelector}
			}
			if p.NamespaceSelector != nil {
				peer.NamespaceSelector = &metav1.LabelSelector{MatchLabels: *p.NamespaceSelector}
			}
			from = append(from, peer)
		}
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyName(crName),
			Namespace: namespace,
			Labels:    CommonLabels(crName, "valkey"),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: SelectorLabels(crName)},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{From: from, Ports: ports},
			},
		},
	}
}

func portRef(p int32) *intstr.IntOrString {
	v := intstr.FromInt(int(p))
	return &v
}
