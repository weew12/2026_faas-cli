package schema

// KubernetesSecret Kubernetes Secret 完整结构
type KubernetesSecret struct {
	Kind       string                   `json:"kind"`
	ApiVersion string                   `json:"apiVersion"`
	Metadata   KubernetesSecretMetadata `json:"metadata"`
	Data       map[string]string        `json:"data"`
}

// KubernetesSecretMetadata Secret 元数据结构
type KubernetesSecretMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
