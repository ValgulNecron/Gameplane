package oci

import "sigs.k8s.io/yaml"

// unmarshalYAML decodes a YAML document into out using JSON tags. We
// reuse k8s sigs/yaml so module.yaml's `json:"name"` tags work the same
// way they do for K8s objects.
func unmarshalYAML(data []byte, out any) error { return yaml.Unmarshal(data, out) }
