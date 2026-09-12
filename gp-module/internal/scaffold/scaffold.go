package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ValgulNecron/gameplane/gp-module/internal/archetypes"
	"github.com/ValgulNecron/gameplane/gp-module/internal/common"
	"gopkg.in/yaml.v3"
)

// ScaffoldOptions defines inputs for module scaffolding.
type ScaffoldOptions struct {
	Name             string
	DisplayName      string
	Archetype        string
	Image            string
	Ports            []archetypes.PortDef
	StorageSize      string
	StorageMountPath string
	Categories       []string
	Summary          string
	OutputDir        string
	Overwrite        bool
}

// GeneratedFiles holds the in-memory contents of generated module files.
type GeneratedFiles struct {
	ModuleYAML   string `json:"moduleYaml"`
	TemplateYAML string `json:"templateYaml"`
	ReadmeMD     string `json:"readmeMd"`
	IconBytes    []byte `json:"-"`
	IconBase64   string `json:"iconBase64,omitempty"`
}

// ScaffoldResult holds the result of scaffolding a module to disk.
type ScaffoldResult struct {
	Dir          string
	Files        GeneratedFiles
	CreatedFiles []string
}

// GenerateFiles generates the module files in-memory from options and archetype defaults.
func GenerateFiles(opts ScaffoldOptions) (*GeneratedFiles, error) {
	if err := common.ValidateModuleName(opts.Name); err != nil {
		return nil, err
	}

	archetypeID := opts.Archetype
	if archetypeID == "" {
		archetypeID = "generic"
	}
	arch, err := archetypes.GetArchetype(archetypeID)
	if err != nil {
		return nil, err
	}

	displayName := opts.DisplayName
	if displayName == "" {
		displayName = titleize(opts.Name)
	}

	image := opts.Image
	if image == "" {
		image = arch.DefaultImage
	}

	ports := opts.Ports
	if len(ports) == 0 {
		ports = arch.DefaultPorts
	}

	storageSize := opts.StorageSize
	if storageSize == "" {
		storageSize = arch.DefaultStorage.Size
	}

	storageMount := opts.StorageMountPath
	if storageMount == "" {
		storageMount = arch.DefaultStorage.MountPath
	}

	categories := opts.Categories
	if len(categories) == 0 {
		categories = arch.DefaultCategories
	}

	summary := opts.Summary
	if summary == "" {
		summary = fmt.Sprintf("Dedicated server for %s", displayName)
	}

	// 1. Render module.yaml
	moduleDoc := map[string]any{
		"apiVersion":  "gameplane.local/module/v1",
		"name":        opts.Name,
		"displayName": displayName,
		"version":     "1.0.0",
		"game":        opts.Name,
		"categories":  categories,
		"summary":     summary,
		"homepage":    "",
		"license":     "MIT",
		"icon":        "icon.png",
	}

	var moduleBuf strings.Builder
	moduleBuf.WriteString("# yaml-language-server: $schema=../.schema/module.schema.json\n")
	enc := yaml.NewEncoder(&moduleBuf)
	enc.SetIndent(2)
	if err := enc.Encode(moduleDoc); err != nil {
		return nil, fmt.Errorf("failed to encode module.yaml: %w", err)
	}

	// 2. Render template.yaml
	specDoc := map[string]any{
		"displayName": displayName,
		"game":        opts.Name,
		"version":     "1.0.0",
		"categories":  categories,
		"description": fmt.Sprintf("Dedicated game server for %s managed by Gameplane.\n", displayName),
		"image":       image,
		"ports":       ports,
		"storage": map[string]string{
			"size":      storageSize,
			"mountPath": storageMount,
		},
	}

	if len(arch.DefaultEnv) > 0 {
		specDoc["env"] = arch.DefaultEnv
	}
	if len(arch.ConfigSchema) > 0 {
		specDoc["configSchema"] = arch.ConfigSchema
	}
	if len(arch.Capabilities.Lifecycle.Stop) > 0 {
		specDoc["capabilities"] = arch.Capabilities
	}

	templateDoc := map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata": map[string]any{
			"name": opts.Name,
			"labels": map[string]string{
				"gameplane.local/module": opts.Name,
			},
		},
		"spec": specDoc,
	}

	var templateBuf strings.Builder
	templateBuf.WriteString("# yaml-language-server: $schema=../.schema/gametemplate.schema.json\n")
	encTmpl := yaml.NewEncoder(&templateBuf)
	encTmpl.SetIndent(2)
	if err := encTmpl.Encode(templateDoc); err != nil {
		return nil, fmt.Errorf("failed to encode template.yaml: %w", err)
	}

	// 3. Render README.md
	var readmeBuf strings.Builder
	readmeBuf.WriteString(fmt.Sprintf("# %s\n\n", displayName))
	readmeBuf.WriteString(fmt.Sprintf("%s\n\n", summary))
	readmeBuf.WriteString("## Overview\n\n")
	readmeBuf.WriteString(fmt.Sprintf("This module packages the dedicated server for **%s** as a Gameplane OCI bundle.\n\n", displayName))
	readmeBuf.WriteString("## Networking & Ports\n\n")
	for _, p := range ports {
		adv := "internal only"
		if p.Advertise {
			adv = "publicly advertised"
		}
		readmeBuf.WriteString(fmt.Sprintf("- **%s**: port `%d/%s` (%s)\n", p.Name, p.ContainerPort, p.Protocol, adv))
	}
	readmeBuf.WriteString("\n## Storage\n\n")
	readmeBuf.WriteString(fmt.Sprintf("- Volume size: `%s`\n- Mount path: `%s`\n", storageSize, storageMount))

	icon := archetypes.PlaceholderIconBytes()

	return &GeneratedFiles{
		ModuleYAML:   moduleBuf.String(),
		TemplateYAML: templateBuf.String(),
		ReadmeMD:     readmeBuf.String(),
		IconBytes:    icon,
	}, nil
}

// Scaffold generates and writes module files to target directory.
func Scaffold(opts ScaffoldOptions) (*ScaffoldResult, error) {
	outDir := opts.OutputDir
	if outDir == "" {
		outDir = filepath.Join("modules", opts.Name)
	}

	if fi, err := os.Stat(outDir); err == nil && fi.IsDir() {
		if !opts.Overwrite {
			return nil, fmt.Errorf("target directory %q already exists; use --overwrite (-f) to overwrite", outDir)
		}
	}

	files, err := GenerateFiles(opts)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(outDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory %q: %w", outDir, err)
	}

	created := []string{}

	modulePath := filepath.Join(outDir, "module.yaml")
	if err := os.WriteFile(modulePath, []byte(files.ModuleYAML), 0600); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", modulePath, err)
	}
	created = append(created, modulePath)

	templatePath := filepath.Join(outDir, "template.yaml")
	if err := os.WriteFile(templatePath, []byte(files.TemplateYAML), 0600); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", templatePath, err)
	}
	created = append(created, templatePath)

	readmePath := filepath.Join(outDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(files.ReadmeMD), 0600); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", readmePath, err)
	}
	created = append(created, readmePath)

	iconPath := filepath.Join(outDir, "icon.png")
	if err := os.WriteFile(iconPath, files.IconBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", iconPath, err)
	}
	created = append(created, iconPath)

	return &ScaffoldResult{
		Dir:          outDir,
		Files:        *files,
		CreatedFiles: created,
	}, nil
}

func titleize(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}
