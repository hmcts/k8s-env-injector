package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

const (
	admissionWebhookAnnotationInjectKey = "env-injector-webhook-inject"
	admissionWebhookAnnotationStatusKey = "env-injector-webhook-status"
)

type WebhookServer struct {
	envConfig *Config
	server    *http.Server
}

// Webhook Server parameters
type WhSvrParameters struct {
	port       int    // webhook server port
	certFile   string // path to the x509 certificate for https
	keyFile    string // path to the x509 private key matching `CertFile`
	envCfgFile string // path to env injector configuration file
}

type Config struct {
	Env                        []corev1.EnvVar                   `yaml:"env"`
	DnsOptions                 []corev1.PodDNSConfigOption       `yaml:"dnsOptions,omitempty"`
	RequiredNodeAffinityTerms  []corev1.NodeSelectorTerm         `yaml:"requiredNodeAffinityTerms,omitempty"`
	PreferredNodeAffinityTerms []corev1.PreferredSchedulingTerm  `yaml:"preferredNodeAffinityTerms,omitempty"`
	Tolerations                []corev1.Toleration               `yaml:"tolerations,omitempty"`
	TopologyConstraints        []corev1.TopologySpreadConstraint `yaml:"topologyConstraints,omitempty"`
	RemovePodAntiAffinity      bool                              `yaml:"removePodAntiAffinity,omitempty"`
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1.AddToScheme(runtimeScheme)
}

func loadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	glog.Infof("New configuration: sha256sum %x", sha256.Sum256(data))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	glog.Infof("Configuration data: %+v", &cfg)

	return &cfg, nil
}

// mutationRequired checks whether the target resource needs to be mutated.
// Mutation is enabled by default unless explicitly disabled.
func mutationRequired(ignoredList []string, metadata *metav1.ObjectMeta) bool {
	// skip excluded kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			glog.Infof("Skip mutation for %v in namespace: %v", metadata.Name, metadata.Namespace)
			return false
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	status := annotations[admissionWebhookAnnotationStatusKey]

	// determine whether to perform mutation based on annotation for the target resource
	var required bool
	if strings.ToLower(status) == "injected" {
		required = false
	} else {
		switch strings.ToLower(annotations[admissionWebhookAnnotationInjectKey]) {
		default:
			required = true
		case "n", "no", "false", "off":
			required = false
		}
	}

	glog.Infof("Mutation policy for %v/%v: status: %q required:%v", metadata.Namespace, metadata.Name, status, required)
	return required
}

// addEnv performs the mutation(s) needed to add the extra environment variables to the target
// resource
func addEnv(target, envVars []corev1.EnvVar, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, envVar := range envVars {
		value = envVar
		path := basePath
		op := "add"
		if first {
			first = false
			value = []corev1.EnvVar{envVar}
		} else {
			optExists := false
			for idx, targetOpt := range target {
				if targetOpt.Name == envVar.Name {
					optExists = true
					op = "replace"
					path = fmt.Sprintf("%s/%d", path, idx)
					break
				}
			}
			if !optExists {
				path = path + "/-"
			}
		}
		patch = append(patch, patchOperation{
			Op:    op,
			Path:  path,
			Value: value,
		})
	}
	return patch
}

// addDnsOptions performs the mutation(s) needed to add the extra dnsOptions to the target
// resource
func addDnsOptions(target, dnsOptions []corev1.PodDNSConfigOption, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, dnsOpt := range dnsOptions {
		value = dnsOpt
		path := basePath
		op := "add"
		if first {
			first = false
			value = []corev1.PodDNSConfigOption{dnsOpt}
		} else {
			optExists := false
			for idx, targetOpt := range target {
				if targetOpt.Name == dnsOpt.Name {
					optExists = true
					op = "replace"
					path = fmt.Sprintf("%s/%d", path, idx)
					break
				}
			}
			if !optExists {
				path = path + "/-"
			}
		}
		patch = append(patch, patchOperation{
			Op:    op,
			Path:  path,
			Value: value,
		})
	}
	return patch
}

// addRequiredNodeAffinityTerms performs the mutation(s) needed to add selector terms to the node affinity
// RequiredDuringSchedulingIgnoredDuringExecution section of to the target resource
func addRequiredNodeAffinityTerms(target, requiredNodeAffinityTerms []corev1.NodeSelectorTerm, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for idx, rna := range requiredNodeAffinityTerms {
		value = rna
		path := basePath
		skip := false
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.NodeSelectorTerm{rna}
		} else {
			optExists := false
			for i, targetOpt := range target {
				if len(targetOpt.MatchExpressions) > 0 {

					matchExpr := targetOpt.MatchExpressions[idx]
					rnaMatchExpr := rna.MatchExpressions[idx]
					keyEqual := cmp.Equal(matchExpr.Key, rnaMatchExpr.Key)
					if keyEqual {
						operatorEqual := cmp.Equal(matchExpr.Operator, rnaMatchExpr.Operator)
						valuesEqual := cmp.Equal(matchExpr.Values, rnaMatchExpr.Values)
						if !operatorEqual || !valuesEqual {
							optExists = true
							op = "replace"
							path = fmt.Sprintf("%s/%d", path, i)
						} else {
							optExists = true
							skip = true
							break
						}
					}
				}

				if len(targetOpt.MatchFields) > 0 {
					matchFlds := targetOpt.MatchFields[idx]
					rnamatchFlds := rna.MatchFields[idx]
					keyEqual := cmp.Equal(matchFlds.Key, rnamatchFlds.Key)
					if keyEqual {
						operatorEqual := cmp.Equal(matchFlds.Operator, rnamatchFlds.Operator)
						valuesEqual := cmp.Equal(matchFlds.Values, rnamatchFlds.Values)
						if !operatorEqual || !valuesEqual {
							optExists = true
							op = "replace"
							path = fmt.Sprintf("%s/%d", path, i)
						} else {
							optExists = true
							skip = true
							break
						}
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
	for idx, pna := range preferredNodeAffinityTerms {
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
			for i, targetOpt := range target {
				if len(targetOpt.Preference.MatchExpressions) > 0 {

					matchExpr := targetOpt.Preference.MatchExpressions[idx]
					pnaMatchExpr := pna.Preference.MatchExpressions[idx]
					keyEqual := cmp.Equal(matchExpr.Key, pnaMatchExpr.Key)
					if keyEqual {
						operatorEqual := cmp.Equal(matchExpr.Operator, pnaMatchExpr.Operator)
						valuesEqual := cmp.Equal(matchExpr.Values, pnaMatchExpr.Values)
						weightEqual := cmp.Equal(targetOpt.Weight, pna.Weight)
						if !operatorEqual || !valuesEqual || !weightEqual {
							optExists = true
							op = "replace"
							path = fmt.Sprintf("%s/%d", path, i)
						} else {
							optExists = true
							skip = true
							break
						}
					}
				}

				if len(targetOpt.Preference.MatchFields) > 0 {

					matchExpr := targetOpt.Preference.MatchFields[idx]
					pnaMatchExpr := pna.Preference.MatchFields[idx]
					keyEqual := cmp.Equal(matchExpr.Key, pnaMatchExpr.Key)
					if keyEqual {
						operatorEqual := cmp.Equal(matchExpr.Operator, pnaMatchExpr.Operator)
						valuesEqual := cmp.Equal(matchExpr.Values, pnaMatchExpr.Values)
						weightEqual := cmp.Equal(targetOpt.Weight, pna.Weight)
						if !operatorEqual || !valuesEqual || !weightEqual {
							optExists = true
							op = "replace"
							path = fmt.Sprintf("%s/%d", path, i)
						} else {
							optExists = true
							skip = true
							break
						}
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

// addToleration performs the mutation(s) needed to add the extra tolerations to the target resource
func addTolerations(target, Tolerations []corev1.Toleration, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, tol := range Tolerations {
		value = tol
		path := basePath
		skip := false
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.Toleration{tol}
		} else {
			optExists := false
			for i, targetOpt := range target {
				keyEqual := cmp.Equal(targetOpt.Key, tol.Key)
				if keyEqual {
					operatorEqual := cmp.Equal(targetOpt.Operator, tol.Operator)
					effectEqual := cmp.Equal(targetOpt.Effect, tol.Effect)
					valueEqual := cmp.Equal(targetOpt.Value, tol.Value)

					if !operatorEqual || !valueEqual || !effectEqual {
						optExists = true
						op = "replace"
						path = fmt.Sprintf("%s/%d", path, i)
					} else {
						optExists = true
						skip = true
						break
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

// addTopologySpreadConstraints performs the mutation(s) needed to add Topology Spread Constraints to your resource
func addTopologySpreadConstraints(target, TopologyConstraints []corev1.TopologySpreadConstraint, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, tsc := range TopologyConstraints {
		value = tsc
		path := basePath
		skip := false
		var op string
		if first {
			first = false
			op = "add"
			value = []corev1.TopologySpreadConstraint{tsc}
		} else {
			optExists := false
			for i, targetOpt := range target {

				keyEqual := cmp.Equal(targetOpt.TopologyKey, tsc.TopologyKey)
				if keyEqual {
					skewEqual := cmp.Equal(targetOpt.MaxSkew, tsc.MaxSkew)
					nodeAffinityEqual := cmp.Equal(targetOpt.NodeAffinityPolicy, tsc.NodeAffinityPolicy)
					nodeTaintEqual := cmp.Equal(targetOpt.NodeTaintsPolicy, tsc.NodeTaintsPolicy)
					unsatisfiableEqual := cmp.Equal(targetOpt.WhenUnsatisfiable, tsc.WhenUnsatisfiable)
					labelSelectorEqual := cmp.Equal(targetOpt.LabelSelector, tsc.LabelSelector)
					matchLabelKeysEqual := cmp.Equal(targetOpt.MatchLabelKeys, tsc.MatchLabelKeys)

					if !skewEqual || !nodeAffinityEqual || !nodeTaintEqual || !unsatisfiableEqual || !labelSelectorEqual || !matchLabelKeysEqual {
						optExists = true
						op = "replace"
						path = fmt.Sprintf("%s/%d", path, i)
					} else {
						optExists = true
						skip = true
						break
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

// removePodAntiAffinity performs the mutation(s) needed to remove podAntiAffinity
func removePodAntiAffinity(basePath string) (patch []patchOperation) {
	patch = append(patch, patchOperation{
		Op:   "remove",
		Path: basePath,
	})

	return patch
}

func updateAnnotation(target map[string]string, annotations map[string]string) (patch []patchOperation) {
	for k, v := range annotations {
		if target == nil {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					k: v,
				},
			})
		} else if target[k] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/metadata/annotations/" + k,
				Value: v,
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + k,
				Value: v,
			})
		}
	}
	return patch
}

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

// main mutation process
func (whsvr *WebhookServer) mutate(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	req := ar.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	// determine whether to perform mutation
	if !mutationRequired(ignoredNamespaces, &pod.ObjectMeta) {
		glog.Infof("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		return &v1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := map[string]string{admissionWebhookAnnotationStatusKey: "injected"}
	patchBytes, err := createPatch(&pod, whsvr.envConfig, annotations)
	if err != nil {
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	return &v1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1.PatchType {
			pt := v1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// serve manages requests to the webhook server
func (whsvr *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1.AdmissionResponse
	ar := v1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(&ar)
	}

	admissionReview := v1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
