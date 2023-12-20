package main

// removePodAntiAffinity performs the mutation(s) needed to remove podAntiAffinity
func removePodAntiAffinity(basePath string) (patch []patchOperation) {
	patch = append(patch, patchOperation{
		Op:   "remove",
		Path: basePath,
	})

	return patch
}
