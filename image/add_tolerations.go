package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addToleration performs the mutation(s) needed to add the extra tolerations to the target resource
func addTolerations(target, Tolerations []corev1.Toleration, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, tol := range Tolerations {
		value = tol
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.Toleration{tol}
		} else {
			optExists := false
			for idx, targetOpt := range target {
				keyEqual := cmp.Equal(targetOpt.Key, tol.Key)
				if keyEqual {
					optExists = true
					operatorEqual := cmp.Equal(targetOpt.Operator, tol.Operator)
					effectEqual := cmp.Equal(targetOpt.Effect, tol.Effect)
					valueEqual := cmp.Equal(targetOpt.Value, tol.Value)

					skip, op, path = checkReplaceOrSkip(idx, path, operatorEqual, effectEqual, valueEqual)
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
