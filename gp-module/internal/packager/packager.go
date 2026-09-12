// Package packager bundles and distributes game module OCI artifacts.
package packager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OCI artifact and media types for module bundles.
const (
	ArtifactType  = "application/vnd.gameplane.module.v1+json"
	MediaMetadata = "application/vnd.gameplane.module.metadata.v1+yaml"
	MediaTemplate = "application/vnd.gameplane.module.template.v1+yaml"
	MediaReadme   = "application/vnd.gameplane.module.readme.v1+md"
	MediaIcon     = "image/png"

	DefaultMaxIconSize   = 512 * 1024  // 512 KiB
	DefaultMaxBundleSize = 1024 * 1024 // 1 MiB
)

// PackageLimits defines size thresholds for assets and bundles.
type PackageLimits struct {
	MaxIconSize   int64
	MaxBundleSize int64
}

// DefaultPackageLimits holds canonical default size limits.
var DefaultPackageLimits = PackageLimits{
	MaxIconSize:   DefaultMaxIconSize,
	MaxBundleSize: DefaultMaxBundleSize,
}

// PackageWarning represents a non-fatal asset or bundle warning.
type PackageWarning struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// PackageOptions configures OCI packaging or tar.gz archive generation.
type PackageOptions struct {
	ModuleDir  string
	OutputFile string
	Registry   string
	Tag        string
	TagLatest  bool
	PlainHTTP  bool
	Insecure   bool
	Limits     PackageLimits
}

// CreateArchiveFromFiles compresses an in-memory map of bundle files into a .tar.gz archive.
func CreateArchiveFromFiles(files map[string][]byte, limits PackageLimits) ([]byte, []PackageWarning, error) {
	required := []string{"module.yaml", "template.yaml", "README.md"}
	for _, req := range required {
		if _, ok := files[req]; !ok {
			return nil, nil, fmt.Errorf("required bundle file %q is missing", req)
		}
	}

	var warnings []PackageWarning
	var totalSize int64

	for name, content := range files {
		size := int64(len(content))
		totalSize += size

		if name == "icon.png" && limits.MaxIconSize > 0 && size > limits.MaxIconSize {
			warnings = append(warnings, PackageWarning{
				File:    "icon.png",
				Message: fmt.Sprintf("icon size (%d bytes) exceeds recommended %d KiB limit", size, limits.MaxIconSize/1024),
			})
		}
	}

	if limits.MaxBundleSize > 0 && totalSize > limits.MaxBundleSize {
		warnings = append(warnings, PackageWarning{
			File:    "bundle",
			Message: fmt.Sprintf("total uncompressed bundle size (%d bytes) exceeds recommended %d KiB limit", totalSize, limits.MaxBundleSize/1024),
		})
	}

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	order := []string{"module.yaml", "template.yaml", "README.md", "icon.png"}
	written := make(map[string]bool)

	writeFile := func(name string, data []byte) error {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
		written[name] = true
		return nil
	}

	for _, name := range order {
		if data, ok := files[name]; ok {
			if err := writeFile(name, data); err != nil {
				return nil, nil, fmt.Errorf("failed to write %s to tar: %w", name, err)
			}
		}
	}

	for name, data := range files {
		if !written[name] {
			if err := writeFile(name, data); err != nil {
				return nil, nil, fmt.Errorf("failed to write %s to tar: %w", name, err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to finalize tar archive: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to finalize gzip compression: %w", err)
	}

	return buf.Bytes(), warnings, nil
}

// CreateArchiveFromDir reads module files from a directory and compresses them into a .tar.gz archive.
func CreateArchiveFromDir(dirPath string, limits PackageLimits) ([]byte, []PackageWarning, error) {
	files, err := ReadModuleFiles(dirPath)
	if err != nil {
		return nil, nil, err
	}
	return CreateArchiveFromFiles(files, limits)
}

// ExportArchiveToFile packages a directory and writes the resulting .tar.gz archive to destination file.
func ExportArchiveToFile(dirPath string, outFile string, limits PackageLimits) ([]PackageWarning, error) {
	archiveBytes, warnings, err := CreateArchiveFromDir(dirPath, limits)
	if err != nil {
		return nil, err
	}

	cleanedOutFile := filepath.Clean(outFile)
	if err := os.MkdirAll(filepath.Dir(cleanedOutFile), 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory for %s: %w", cleanedOutFile, err)
	}

	if err := os.WriteFile(cleanedOutFile, archiveBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write archive to %s: %w", cleanedOutFile, err)
	}

	return warnings, nil
}

// ReadModuleFiles loads standard module files from a directory.
func ReadModuleFiles(dirPath string) (map[string][]byte, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory path: %w", err)
	}

	files := make(map[string][]byte)
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module directory %s: %w", absPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Clean(filepath.Join(absPath, entry.Name()))
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", entry.Name(), err)
		}
		files[entry.Name()] = data
	}

	return files, nil
}

// PushOCI packages and pushes a module directory to an OCI registry using ORAS.
func PushOCI(opts PackageOptions) ([]PackageWarning, error) {
	files, err := ReadModuleFiles(opts.ModuleDir)
	if err != nil {
		return nil, err
	}

	// Verify required layers
	required := []string{"module.yaml", "template.yaml", "README.md"}
	for _, req := range required {
		if _, ok := files[req]; !ok {
			return nil, fmt.Errorf("required file %s is missing in module directory", req)
		}
	}

	// Size checks
	var warnings []PackageWarning
	var totalSize int64
	for name, content := range files {
		size := int64(len(content))
		totalSize += size
		if name == "icon.png" && opts.Limits.MaxIconSize > 0 && size > opts.Limits.MaxIconSize {
			warnings = append(warnings, PackageWarning{
				File:    "icon.png",
				Message: fmt.Sprintf("icon size (%d bytes) exceeds recommended %d KiB limit", size, opts.Limits.MaxIconSize/1024),
			})
		}
	}
	if opts.Limits.MaxBundleSize > 0 && totalSize > opts.Limits.MaxBundleSize {
		warnings = append(warnings, PackageWarning{
			File:    "bundle",
			Message: fmt.Sprintf("total module size (%d bytes) exceeds recommended %d KiB limit", totalSize, opts.Limits.MaxBundleSize/1024),
		})
	}

	// Read module metadata for name and version
	var meta map[string]any
	if err := yaml.Unmarshal(files["module.yaml"], &meta); err != nil {
		return nil, fmt.Errorf("failed to parse module.yaml: %w", err)
	}

	modName, _ := meta["name"].(string)
	modVersion, _ := meta["version"].(string)
	if modName == "" {
		return nil, fmt.Errorf("module.yaml is missing name")
	}

	tag := opts.Tag
	if tag == "" {
		tag = modVersion
	}
	if tag == "" {
		return nil, fmt.Errorf("module.yaml is missing version and no --tag was provided")
	}

	if _, err := exec.LookPath("oras"); err != nil {
		return warnings, fmt.Errorf("oras binary not found in PATH; install ORAS (https://oras.land) or use --output <path> to generate a local .tar.gz bundle")
	}

	targetRef := fmt.Sprintf("%s/%s:%s", strings.TrimRight(opts.Registry, "/"), modName, tag)

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "push")
	if opts.PlainHTTP {
		cmdArgs = append(cmdArgs, "--plain-http")
	}
	if opts.Insecure {
		cmdArgs = append(cmdArgs, "--insecure")
	}
	cmdArgs = append(cmdArgs, "--artifact-type", ArtifactType, targetRef)

	layerArgs := []string{
		fmt.Sprintf("module.yaml:%s", MediaMetadata),
		fmt.Sprintf("template.yaml:%s", MediaTemplate),
	}
	if _, ok := files["README.md"]; ok {
		layerArgs = append(layerArgs, fmt.Sprintf("README.md:%s", MediaReadme))
	}
	if _, ok := files["icon.png"]; ok {
		layerArgs = append(layerArgs, fmt.Sprintf("icon.png:%s", MediaIcon))
	}
	cmdArgs = append(cmdArgs, layerArgs...)

	cmd := exec.CommandContext(context.Background(), "oras", cmdArgs...)
	cmd.Dir = opts.ModuleDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return warnings, fmt.Errorf("oras push failed: %w", err)
	}

	if opts.TagLatest {
		latestRef := fmt.Sprintf("%s/%s:latest", strings.TrimRight(opts.Registry, "/"), modName)
		cmdLatest := exec.CommandContext(context.Background(), "oras", "tag")
		if opts.PlainHTTP {
			cmdLatest.Args = append(cmdLatest.Args, "--plain-http")
		}
		if opts.Insecure {
			cmdLatest.Args = append(cmdLatest.Args, "--insecure")
		}
		cmdLatest.Args = append(cmdLatest.Args, targetRef, "latest")
		cmdLatest.Stdout = os.Stdout
		cmdLatest.Stderr = os.Stderr
		if err := cmdLatest.Run(); err != nil {
			warnings = append(warnings, PackageWarning{
				File:    latestRef,
				Message: fmt.Sprintf("failed to tag :latest: %v", err),
			})
		}
	}

	return warnings, nil
}
