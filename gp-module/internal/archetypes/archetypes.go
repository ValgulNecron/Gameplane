package archetypes

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"sync"
)

// PortDef represents a port configuration in an archetype.
type PortDef struct {
	Name          string `json:"name" yaml:"name"`
	ContainerPort int    `json:"containerPort" yaml:"containerPort"`
	Protocol      string `json:"protocol" yaml:"protocol"`
	Advertise     bool   `json:"advertise" yaml:"advertise"`
}

// StorageDef represents storage configuration in an archetype.
type StorageDef struct {
	Size      string `json:"size" yaml:"size"`
	MountPath string `json:"mountPath" yaml:"mountPath"`
}

// EnvDef represents an environment variable declaration.
type EnvDef struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// AutoMemoryDef represents autoFromMemoryLimit configuration.
type AutoMemoryDef struct {
	Percent int `json:"percent" yaml:"percent"`
}

// ConfigFieldDef represents a configSchema field declaration.
type ConfigFieldDef struct {
	Name                string         `json:"name" yaml:"name"`
	DisplayName         string         `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description         string         `json:"description,omitempty" yaml:"description,omitempty"`
	Type                string         `json:"type" yaml:"type"`
	Default             string         `json:"default,omitempty" yaml:"default,omitempty"`
	Required            bool           `json:"required,omitempty" yaml:"required,omitempty"`
	Min                 *int64         `json:"min,omitempty" yaml:"min,omitempty"`
	Max                 *int64         `json:"max,omitempty" yaml:"max,omitempty"`
	AutoFromMemoryLimit *AutoMemoryDef `json:"autoFromMemoryLimit,omitempty" yaml:"autoFromMemoryLimit,omitempty"`
}

// CapabilitiesDef represents lifecycle and management capabilities.
type CapabilitiesDef struct {
	Lifecycle LifecycleDef `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
}

// LifecycleDef represents stop command sequences.
type LifecycleDef struct {
	Stop []string `json:"stop,omitempty" yaml:"stop,omitempty"`
}

// ArchetypeDefinition represents a starter template archetype.
type ArchetypeDefinition struct {
	ID                string           `json:"id"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	DefaultImage      string           `json:"defaultImage"`
	DefaultPorts      []PortDef        `json:"defaultPorts"`
	DefaultStorage    StorageDef       `json:"defaultStorage"`
	DefaultEnv        []EnvDef         `json:"defaultEnv"`
	ConfigSchema      []ConfigFieldDef `json:"configSchema"`
	Capabilities      CapabilitiesDef  `json:"capabilities"`
	DefaultCategories []string         `json:"defaultCategories"`
}

var (
	iconOnce  sync.Once
	iconBytes []byte
)

// PlaceholderIconBytes returns a valid 256x256 RGBA PNG placeholder icon.
func PlaceholderIconBytes() []byte {
	iconOnce.Do(func() {
		img := image.NewRGBA(image.Rect(0, 0, 256, 256))
		// Gradient / dark slate background #1e293b
		bgColor := color.RGBA{R: 30, G: 41, B: 59, A: 255}
		draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

		// Inner badge #3b82f6
		badgeColor := color.RGBA{R: 59, G: 130, B: 246, A: 255}
		innerRect := image.Rect(48, 48, 208, 208)
		draw.Draw(img, innerRect, &image.Uniform{C: badgeColor}, image.Point{}, draw.Over)

		// Accent center #ffffff
		whiteColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		centerRect := image.Rect(96, 96, 160, 160)
		draw.Draw(img, centerRect, &image.Uniform{C: whiteColor}, image.Point{}, draw.Over)

		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			panic(fmt.Sprintf("failed to encode placeholder PNG: %v", err))
		}
		iconBytes = buf.Bytes()
	})

	cp := make([]byte, len(iconBytes))
	copy(cp, iconBytes)
	return cp
}

// AllArchetypes returns the map of supported archetypes.
func AllArchetypes() map[string]ArchetypeDefinition {
	minPlayer := int64(1)
	maxPlayer := int64(128)

	return map[string]ArchetypeDefinition{
		"steamcmd": {
			ID:           "steamcmd",
			Title:        "SteamCMD Dedicated Server",
			Description:  "Dedicated game server installed and managed via SteamCMD (Valve UDP ports, save volume, non-root user)",
			DefaultImage: "cm2network/steamcmd:root@sha256:4d830b0475b8719f96b9978ba57404434bb3da3f260388d75cfb373cf5889ea8",
			DefaultPorts: []PortDef{
				{Name: "game", ContainerPort: 27015, Protocol: "UDP", Advertise: true},
				{Name: "query", ContainerPort: 27016, Protocol: "UDP", Advertise: true},
			},
			DefaultStorage: StorageDef{
				Size:      "20Gi",
				MountPath: "/serverdata",
			},
			DefaultEnv: []EnvDef{
				{Name: "STEAMAPPID", Value: "0"},
				{Name: "SERVER_NAME", Value: "Game Server"},
			},
			ConfigSchema: []ConfigFieldDef{
				{
					Name:        "SERVER_PASSWORD",
					DisplayName: "Server Password",
					Description: "Password required for players to join the server",
					Type:        "password",
					Required:    false,
				},
				{
					Name:        "MAX_PLAYERS",
					DisplayName: "Maximum Players",
					Description: "Maximum allowed concurrent players",
					Type:        "int",
					Default:     "16",
					Min:         &minPlayer,
					Max:         &maxPlayer,
				},
			},
			Capabilities: CapabilitiesDef{
				Lifecycle: LifecycleDef{
					Stop: []string{"quit"},
				},
			},
			DefaultCategories: []string{"Survival", "Co-op"},
		},
		"java": {
			ID:           "java",
			Title:        "Java Application Server",
			Description:  "JVM-based game server (Minecraft, etc.) with automatic memory heap calculation and RCON support",
			DefaultImage: "eclipse-temurin:21-jre-jammy@sha256:0d5bba8111956f2f01fbf9c054238e55e09f58ea9fc3a3b5c6e838ebdcbb636d",
			DefaultPorts: []PortDef{
				{Name: "game", ContainerPort: 25565, Protocol: "TCP", Advertise: true},
				{Name: "rcon", ContainerPort: 25575, Protocol: "TCP", Advertise: false},
			},
			DefaultStorage: StorageDef{
				Size:      "10Gi",
				MountPath: "/data",
			},
			DefaultEnv: []EnvDef{
				{Name: "EULA", Value: "TRUE"},
				{Name: "ENABLE_RCON", Value: "true"},
				{Name: "RCON_PORT", Value: "25575"},
			},
			ConfigSchema: []ConfigFieldDef{
				{
					Name:        "MAX_MEMORY",
					DisplayName: "Maximum JVM Heap Memory",
					Description: "Maximum memory allocated to JVM, dynamically calculated from container limit",
					Type:        "string",
					AutoFromMemoryLimit: &AutoMemoryDef{
						Percent: 75,
					},
				},
				{
					Name:        "RCON_PASSWORD",
					DisplayName: "RCON Password",
					Description: "Remote console management password",
					Type:        "password",
					Required:    false,
				},
			},
			Capabilities: CapabilitiesDef{
				Lifecycle: LifecycleDef{
					Stop: []string{"stop"},
				},
			},
			DefaultCategories: []string{"Sandbox", "Survival"},
		},
		"generic": {
			ID:           "generic",
			Title:        "Generic Container Server",
			Description:  "General-purpose containerized game server with configurable TCP/UDP ports and persistent storage",
			DefaultImage: "alpine:3.20@sha256:b89d9c10e96b8da1b4874828d0a479b157d60b49364fae0db5122e1b5056f918",
			DefaultPorts: []PortDef{
				{Name: "game", ContainerPort: 8080, Protocol: "TCP", Advertise: true},
			},
			DefaultStorage: StorageDef{
				Size:      "5Gi",
				MountPath: "/data",
			},
			DefaultEnv:        []EnvDef{},
			ConfigSchema:      []ConfigFieldDef{},
			Capabilities:      CapabilitiesDef{},
			DefaultCategories: []string{"Co-op"},
		},
	}
}

// GetArchetype returns the archetype definition for id, or an error if unknown.
func GetArchetype(id string) (*ArchetypeDefinition, error) {
	archs := AllArchetypes()
	normalized := strings.ToLower(strings.TrimSpace(id))
	arch, ok := archs[normalized]
	if !ok {
		return nil, fmt.Errorf("unknown archetype %q: available archetypes are [steamcmd, java, generic]", id)
	}
	return &arch, nil
}
