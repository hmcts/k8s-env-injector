package main

import (
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

// addDnsOptions performs the mutation(s) needed to add the extra dnsOptions to the target
// resource
func addDnsOptions(target, dnsOptions []corev1.PodDNSConfigOption, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, dnsOpt := range dnsOptions {
		value = dnsOpt
		path := basePath
		var skip bool
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.PodDNSConfigOption{dnsOpt}
		} else {
			optExists := false
			for idx, targetOpt := range target {
				nameEqual := cmp.Equal(targetOpt.Name, dnsOpt.Name)
				if nameEqual {
					optExists = true
					valueEqual := cmp.Equal(targetOpt.Value, dnsOpt.Value)

					skip, op, path = checkReplaceOrSkip(idx, path, valueEqual)
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
