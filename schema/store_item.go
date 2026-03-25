package schema

// StoreItem 表示应用商店中的一个函数项
type StoreItem struct {
	Icon                   string            `json:"icon"`
	Title                  string            `json:"title"`
	Description            string            `json:"description"`
	Image                  string            `json:"image"`
	Name                   string            `json:"name"`
	Fprocess               string            `json:"fprocess"`
	Network                string            `json:"network"`
	RepoURL                string            `json:"repo_url"`
	Environment            map[string]string `json:"environment"`
	Labels                 map[string]string `json:"labels"`
	Annotations            map[string]string `json:"annotations"`
	ReadOnlyRootFilesystem bool              `json:"readOnlyRootFilesystem"`
}
