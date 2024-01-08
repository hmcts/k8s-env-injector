package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addEnv performs the mutation(s) needed to add the extra environment variables to the target
// resource
func addEnv(target, envVars []corev1.EnvVar, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, envVar := range envVars {
		value = envVar
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.EnvVar{envVar}
		} else {

			optExists := false
			for idx, targetOpt := range target {
				nameEqual := cmp.Equal(targetOpt.Name, envVar.Name)
				if nameEqual {
					optExists = true
					valueEqual := cmp.Equal(targetOpt.Value, envVar.Value)
					valueFromEqual := cmp.Equal(targetOpt.ValueFrom, envVar.ValueFrom)

					skip, op, path = checkReplaceOrSkip(idx, path, valueEqual, valueFromEqual)
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
