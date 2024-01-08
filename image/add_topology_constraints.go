package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addTopologySpreadConstraints performs the mutation(s) needed to add Topology Spread Constraints to your resource
func addTopologySpreadConstraints(target, TopologyConstraints []corev1.TopologySpreadConstraint, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, tsc := range TopologyConstraints {
		value = tsc
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.TopologySpreadConstraint{tsc}
		} else {
			optExists := false
			for idx, targetOpt := range target {

				keyEqual := cmp.Equal(targetOpt.TopologyKey, tsc.TopologyKey)
				if keyEqual {
					optExists = true
					skewEqual := cmp.Equal(targetOpt.MaxSkew, tsc.MaxSkew)
					nodeAffinityEqual := cmp.Equal(targetOpt.NodeAffinityPolicy, tsc.NodeAffinityPolicy)
					nodeTaintEqual := cmp.Equal(targetOpt.NodeTaintsPolicy, tsc.NodeTaintsPolicy)
					unsatisfiableEqual := cmp.Equal(targetOpt.WhenUnsatisfiable, tsc.WhenUnsatisfiable)
					labelSelectorEqual := cmp.Equal(targetOpt.LabelSelector, tsc.LabelSelector)
					matchLabelKeysEqual := cmp.Equal(targetOpt.MatchLabelKeys, tsc.MatchLabelKeys)

					skip, op, path = checkReplaceOrSkip(idx, path, skewEqual, nodeAffinityEqual, nodeTaintEqual, unsatisfiableEqual, labelSelectorEqual, matchLabelKeysEqual)
				}
			}
			if !optExists {
				op = "add"
				path = path + "/-"
			}
		}
		if !skip {
			patch = append(patch, patchOperation{
				Op:    op,
				Path:  path,
				Value: value,
			})
		} else {
			patch = []patchOperation{}
		}
	}
	return patch
}
