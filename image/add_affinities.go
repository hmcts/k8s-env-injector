package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addRequiredNodeAffinityTerms performs the mutation(s) needed to add selector terms to the node affinity
// RequiredDuringSchedulingIgnoredDuringExecution section of to the target resource
func addRequiredNodeAffinityTerms(target, requiredNodeAffinityTerms []corev1.NodeSelectorTerm, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for i, rna := range requiredNodeAffinityTerms {
		value = rna
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.NodeSelectorTerm{rna}
		} else {
			optExists := false
			for idx, targetOpt := range target {
				if len(targetOpt.MatchExpressions) > 0 {
					matchExpr := targetOpt.MatchExpressions[i]
					rnaMatchExpr := rna.MatchExpressions[i]
					keyEqual := cmp.Equal(matchExpr.Key, rnaMatchExpr.Key)
					if keyEqual {
						optExists = true
						operatorEqual := cmp.Equal(matchExpr.Operator, rnaMatchExpr.Operator)
						valuesEqual := cmp.Equal(matchExpr.Values, rnaMatchExpr.Values)

						skip, op, path = checkReplaceOrSkip(idx, path, operatorEqual, valuesEqual)

					}
				}

				if len(targetOpt.MatchFields) > 0 {
					matchFlds := targetOpt.MatchFields[i]
					rnamatchFlds := rna.MatchFields[i]
					keyEqual := cmp.Equal(matchFlds.Key, rnamatchFlds.Key)
					if keyEqual {
						optExists = true
						operatorEqual := cmp.Equal(matchFlds.Operator, rnamatchFlds.Operator)
						valuesEqual := cmp.Equal(matchFlds.Values, rnamatchFlds.Values)

						skip, op, path = checkReplaceOrSkip(idx, path, operatorEqual, valuesEqual)
					}
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

// addPreferredNodeAffinityTerms performs the mutation(s) needed to add selector terms to the node affinity
// preferredDuringSchedulingIgnoredDuringExecution section of to the target resource
func addPreferredNodeAffinityTerms(target, preferredNodeAffinityTerms []corev1.PreferredSchedulingTerm, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for i, pna := range preferredNodeAffinityTerms {
		value = pna
		path := basePath
		skip := false
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.PreferredSchedulingTerm{pna}
		} else {
			optExists := false
			for idx, targetOpt := range target {
				if len(targetOpt.Preference.MatchExpressions) > 0 {
					matchExpr := targetOpt.Preference.MatchExpressions[i]
					pnaMatchExpr := pna.Preference.MatchExpressions[i]
					keyEqual := cmp.Equal(matchExpr.Key, pnaMatchExpr.Key)
					if keyEqual {
						optExists = true
						operatorEqual := cmp.Equal(matchExpr.Operator, pnaMatchExpr.Operator)
						valuesEqual := cmp.Equal(matchExpr.Values, pnaMatchExpr.Values)
						weightEqual := cmp.Equal(targetOpt.Weight, pna.Weight)

						skip, op, path = checkReplaceOrSkip(idx, path, operatorEqual, valuesEqual, weightEqual)

					}
				}

				if len(targetOpt.Preference.MatchFields) > 0 {

					matchExpr := targetOpt.Preference.MatchFields[i]
					pnaMatchExpr := pna.Preference.MatchFields[i]
					keyEqual := cmp.Equal(matchExpr.Key, pnaMatchExpr.Key)
					if keyEqual {
						optExists = true
						operatorEqual := cmp.Equal(matchExpr.Operator, pnaMatchExpr.Operator)
						valuesEqual := cmp.Equal(matchExpr.Values, pnaMatchExpr.Values)
						weightEqual := cmp.Equal(targetOpt.Weight, pna.Weight)

						skip, op, path = checkReplaceOrSkip(idx, path, operatorEqual, valuesEqual, weightEqual)
					}
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
