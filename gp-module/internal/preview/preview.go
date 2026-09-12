// Package preview evaluates runtime manifests and dynamic memory calculations.
package preview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Options defines inputs for generating a dry-run preview.
type Options struct {
	ModuleDir    string            `json:"moduleDir,omitempty"`
	TemplateYAML []byte            `json:"templateYaml,omitempty"`
	VersionID    string            `json:"versionId,omitempty"`
	MemoryLimit  string            `json:"memoryLimit,omitempty"`
	UserConfig   map[string]string `json:"userConfig,omitempty"`
}

// PortPreview represents an exposed network port.
type PortPreview struct {
	Name          string `json:"name"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// StoragePreview represents module storage settings.
type StoragePreview struct {
	Size      string `json:"size"`
	MountPath string `json:"mountPath"`
}

// ConfigFieldPreview describes a resolved configuration field with its provenance.
type ConfigFieldPreview struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

// Result holds the synthesized runtime configuration.
type Result struct {
	Game             string               `json:"game"`
	DisplayName      string               `json:"displayName"`
	VersionID        string               `json:"versionId,omitempty"`
	EffectiveImage   string               `json:"effectiveImage"`
	EffectiveMemory  string               `json:"effectiveMemory"`
	ConfigFields     []ConfigFieldPreview `json:"configFields"`
	ComputedConfig   map[string]string    `json:"computedConfig"`
	EffectiveEnv     map[string]string    `json:"effectiveEnv"`
	Ports            []PortPreview        `json:"ports"`
	Storage          StoragePreview       `json:"storage"`
	RenderedManifest string               `json:"renderedManifest,omitempty"`
}

// GeneratePreview evaluates template.yaml against inputs and resolves effective runtime config.
func GeneratePreview(opts Options) (*Result, error) {
	yamlBytes := opts.TemplateYAML
	if len(yamlBytes) == 0 && opts.ModuleDir != "" {
		path := filepath.Join(opts.ModuleDir, "template.yaml")
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
		yamlBytes = data
	}

	if len(yamlBytes) == 0 {
		return nil, fmt.Errorf("no template.yaml content or module directory provided")
	}

	var root map[string]any
	if err := yaml.Unmarshal(yamlBytes, &root); err != nil {
		return nil, fmt.Errorf("failed to parse template.yaml: %w", err)
	}

	spec, _ := root["spec"].(map[string]any)
	if spec == nil {
		return nil, fmt.Errorf("missing spec mapping in template.yaml")
	}

	displayName, _ := spec["displayName"].(string)
	game, _ := spec["game"].(string)
	defaultImage, _ := spec["image"].(string)

	memLimit := opts.MemoryLimit
	if memLimit == "" {
		memLimit = "4Gi"
	}

	effectiveImage := defaultImage
	selectedVer := "default"
	var versionEnv []any

	// Version overlay
	if vers, ok := spec["versions"].([]any); ok && len(vers) > 0 {
		for _, v := range vers {
			vMap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			vid, _ := vMap["id"].(string)
			if opts.VersionID != "" && vid == opts.VersionID {
				selectedVer = vid
				if img, ok := vMap["image"].(string); ok && img != "" {
					effectiveImage = img
				}
				if envList, ok := vMap["env"].([]any); ok {
					versionEnv = envList
				}
				break
			}
		}
	}

	effectiveEnv := make(map[string]string)

	// 1. Base template env
	if baseEnv, ok := spec["env"].([]any); ok {
		for _, e := range baseEnv {
			if eMap, ok := e.(map[string]any); ok {
				k, _ := eMap["name"].(string)
				v := fmt.Sprintf("%v", eMap["value"])
				if k != "" {
					effectiveEnv[k] = v
				}
			}
		}
	}

	// 2. Version overlay env
	for _, e := range versionEnv {
		if eMap, ok := e.(map[string]any); ok {
			k, _ := eMap["name"].(string)
			v := fmt.Sprintf("%v", eMap["value"])
			if k != "" {
				effectiveEnv[k] = v
			}
		}
	}

	computedConfig := make(map[string]string)
	var configFields []ConfigFieldPreview

	// 3. ConfigSchema evaluation
	if schemaList, ok := spec["configSchema"].([]any); ok {
		for _, s := range schemaList {
			sMap, ok := s.(map[string]any)
			if !ok {
				continue
			}
			fieldName, _ := sMap["name"].(string)
			fieldType, _ := sMap["type"].(string)
			if fieldType == "" {
				fieldType = "string"
			}
			defaultVal, _ := sMap["default"].(string)
			target, _ := sMap["target"].(string)
			if target == "" {
				target = "env"
			}

			val := ""
			source := ""

			if userVal, exists := opts.UserConfig[fieldName]; exists && userVal != "" {
				val = userVal
				source = "user override"
			} else if defaultVal != "" {
				val = defaultVal
				source = "default"
			} else if autoMem, ok := sMap["autoFromMemoryLimit"].(map[string]any); ok {
				pct := 0
				switch p := autoMem["percent"].(type) {
				case int:
					pct = p
				case float64:
					pct = int(p)
				}
				if pct > 0 {
					calcVal, err := CalculateAutoMemory(memLimit, pct)
					if err == nil {
						val = calcVal
						source = fmt.Sprintf("autoFromMemoryLimit: %d%% of %s", pct, memLimit)
					}
				}
			}

			if val != "" {
				computedConfig[fieldName] = val
				configFields = append(configFields, ConfigFieldPreview{
					Name:   fieldName,
					Value:  val,
					Type:   fieldType,
					Source: source,
				})

				if target == "env" {
					if fieldType == "password" {
						effectiveEnv[fieldName] = fmt.Sprintf("[secret: configSecret/%s]", fieldName)
					} else {
						effectiveEnv[fieldName] = val
					}
				}
			}
		}
	}

	// Ports
	var ports []PortPreview
	if portsList, ok := spec["ports"].([]any); ok {
		for _, p := range portsList {
			if pMap, ok := p.(map[string]any); ok {
				pName, _ := pMap["name"].(string)
				pPort := 0
				switch num := pMap["containerPort"].(type) {
				case int:
					pPort = num
				case float64:
					pPort = int(num)
				}
				pProto, _ := pMap["protocol"].(string)
				if pProto == "" {
					pProto = "TCP"
				}
				ports = append(ports, PortPreview{
					Name:          pName,
					ContainerPort: pPort,
					Protocol:      pProto,
				})
			}
		}
	}

	// Storage
	storage := StoragePreview{}
	if storMap, ok := spec["storage"].(map[string]any); ok {
		storage.Size, _ = storMap["size"].(string)
		storage.MountPath, _ = storMap["mountPath"].(string)
	}

	return &Result{
		Game:            game,
		DisplayName:     displayName,
		VersionID:       selectedVer,
		EffectiveImage:  effectiveImage,
		EffectiveMemory: memLimit,
		ConfigFields:    configFields,
		ComputedConfig:  computedConfig,
		EffectiveEnv:    effectiveEnv,
		Ports:           ports,
		Storage:         storage,
	}, nil
}

// ToJSON returns indented JSON representation.
func (p *Result) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FormatHuman returns a formatted terminal representation.
func (p *Result) FormatHuman() string {
	var sb strings.Builder

	title := p.DisplayName
	if title == "" {
		title = p.Game
	}
	sb.WriteString(fmt.Sprintf("== %s (%s) Preview ==\n", title, p.Game))
	sb.WriteString(fmt.Sprintf("Selected Version: %s\n", p.VersionID))
	sb.WriteString(fmt.Sprintf("Container Image:  %s\n", p.EffectiveImage))
	sb.WriteString(fmt.Sprintf("Memory Limit:     %s\n\n", p.EffectiveMemory))

	sb.WriteString("Computed Configuration:\n")
	if len(p.ConfigFields) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, cf := range p.ConfigFields {
			sb.WriteString(fmt.Sprintf("  %-16s %s (%s)\n", cf.Name+":", cf.Value, cf.Source))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Effective Environment:\n")
	if len(p.EffectiveEnv) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		// Sort keys for deterministic output
		var keys []string
		for k := range p.EffectiveEnv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("  %-16s %s\n", k+":", p.EffectiveEnv[k]))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Exposed Ports:\n")
	if len(p.Ports) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, port := range p.Ports {
			sb.WriteString(fmt.Sprintf("  - %d/%s (%s)\n", port.ContainerPort, port.Protocol, port.Name))
		}
	}

	return sb.String()
}
