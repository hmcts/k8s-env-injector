package main

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// createPatch creates a mutation patch for resources
func createPatch(pod *corev1.Pod, envConfig *Config, annotations map[string]string) ([]byte, error) {
	var patches []patchOperation

	for idx, container := range pod.Spec.Containers {
		patches = append(patches, addEnv(container.Env, envConfig.Env, fmt.Sprintf("/spec/containers/%d/env", idx))...)
	}
	if len(envConfig.DnsOptions) > 0 {
		if pod.Spec.DNSConfig == nil {
			pod.Spec.DNSConfig = &corev1.PodDNSConfig{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/dnsConfig", Value: corev1.PodDNSConfig{}})
		}
		patches = append(patches, addDnsOptions(pod.Spec.DNSConfig.Options, envConfig.DnsOptions, fmt.Sprintf("/spec/dnsConfig/options"))...)
	}
	if len(envConfig.Tolerations) > 0 {
		if pod.Spec.Tolerations == nil {
			pod.Spec.Tolerations = []corev1.Toleration{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/tolerations", Value: []corev1.Toleration{}})
		}
		patches = append(patches, addTolerations(pod.Spec.Tolerations, envConfig.Tolerations, fmt.Sprintf("/spec/tolerations"))...)
	}
	if len(envConfig.TopologyConstraints) > 0 {
		if pod.Spec.TopologySpreadConstraints == nil {
			pod.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/topologySpreadConstraints", Value: []corev1.TopologySpreadConstraint{}})
		}
		patches = append(patches, addTopologySpreadConstraints(pod.Spec.TopologySpreadConstraints, envConfig.TopologyConstraints, fmt.Sprintf("/spec/topologySpreadConstraints"))...)
	}
	if envConfig.RemovePodAntiAffinity {
		if pod.Spec.Affinity != nil && pod.Spec.Affinity.PodAntiAffinity != nil {
			// Remove PodAntiAffinity
			patches = append(patches, removePodAntiAffinity("/spec/affinity/podAntiAffinity")...)
		}
	}
	if len(envConfig.RequiredNodeAffinityTerms) > 0 {
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &corev1.Affinity{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/affinity", Value: corev1.Affinity{}})
		}
		if pod.Spec.Affinity.NodeAffinity == nil {
			pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/affinity/nodeAffinity", Value: corev1.NodeAffinity{}})
		}
		if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution", Value: corev1.NodeSelector{}})
		}
		patches = append(patches, addRequiredNodeAffinityTerms(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
			envConfig.RequiredNodeAffinityTerms, fmt.Sprintf("/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms"))...)
	}
	if len(envConfig.PreferredNodeAffinityTerms) > 0 {
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &corev1.Affinity{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/affinity", Value: corev1.Affinity{}})
		}
		if pod.Spec.Affinity.NodeAffinity == nil {
			pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/affinity/nodeAffinity", Value: corev1.NodeAffinity{}})
		}
		if pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
			pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.PreferredSchedulingTerm{}
			patches = append(patches, patchOperation{Op: "add", Path: "/spec/affinity/nodeAffinity/preferredDuringSchedulingIgnoredDuringExecution", Value: []corev1.PreferredSchedulingTerm{}})
		}
		patches = append(patches, addPreferredNodeAffinityTerms(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			envConfig.PreferredNodeAffinityTerms, "/spec/affinity/nodeAffinity/preferredDuringSchedulingIgnoredDuringExecution")...)
	}

	patches = append(patches, updateAnnotation(pod.Annotations, annotations)...)

	return json.Marshal(patches)
}
