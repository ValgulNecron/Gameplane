// Package validator provides offline static validation and linting of module manifests.
package validator

import (
	"fmt"
	"net/url"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/ValgulNecron/gameplane/gp-module/internal/common"
)

var (
	semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
	minVerRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?$`)
)

var allowedModuleKeys = map[string]bool{
	"apiVersion":          true,
	"name":                true,
	"displayName":         true,
	"version":             true,
	"game":                true,
	"categories":          true,
	"summary":             true,
	"homepage":            true,
	"license":             true,
	"gameplaneMinVersion": true,
	"icon":                true,
}

// validateModuleSchema checks module.yaml against module.schema.json rules.
func validateModuleSchema(node *yaml.Node) []Finding {
	var findings []Finding

	if node == nil || node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return []Finding{
			{
				Level:       SeverityError,
				RuleID:      RuleMetadataSchemaViolation,
				File:        "module.yaml",
				Line:        1,
				Message:     "module.yaml is empty or malformed YAML",
				Remediation: "Review error line and correct invalid field format according to module.schema.json.",
			},
		}
	}

	rootMap := node.Content[0]
	if rootMap.Kind != yaml.MappingNode {
		return []Finding{
			{
				Level:       SeverityError,
				RuleID:      RuleMetadataSchemaViolation,
				File:        "module.yaml",
				Line:        rootMap.Line,
				Message:     "root of module.yaml must be a mapping",
				Remediation: "Review error line and correct invalid field format according to module.schema.json.",
			},
		}
	}

	// Check unexpected fields
	for i := 0; i < len(rootMap.Content); i += 2 {
		keyNode := rootMap.Content[i]
		if !allowedModuleKeys[keyNode.Value] {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleMetadataSchemaViolation,
				File:        "module.yaml",
				Line:        keyNode.Line,
				Field:       keyNode.Value,
				Message:     fmt.Sprintf("unrecognized field %q in module.yaml", keyNode.Value),
				Remediation: "Review error line and correct invalid field format according to module.schema.json.",
			})
		}
	}

	// Required fields: apiVersion, name, displayName, version, game, summary
	required := []string{"apiVersion", "name", "displayName", "version", "game", "summary"}
	for _, req := range required {
		kn := common.FindKeyNode(rootMap, req)
		vn := common.FindNode(node, req)
		if kn == nil || vn == nil {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleMetadataSchemaViolation,
				File:        "module.yaml",
				Line:        1,
				Field:       req,
				Message:     fmt.Sprintf("missing required field %q", req),
				Remediation: "Review error line and correct invalid field format according to module.schema.json.",
			})
		}
	}

	// Check apiVersion const
	apiVerNode := common.FindNode(node, "apiVersion")
	if apiVerNode != nil {
		if apiVerNode.Value != "gameplane.local/module/v1" {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleMetadataSchemaViolation,
				File:        "module.yaml",
				Line:        apiVerNode.Line,
				Field:       "apiVersion",
				Message:     fmt.Sprintf("apiVersion must be %q, got %q", "gameplane.local/module/v1", apiVerNode.Value),
				Remediation: "Review error line and correct invalid field format according to module.schema.json.",
			})
		}
	}

	// Check displayName minLength
	dispNode := common.FindNode(node, "displayName")
	if dispNode != nil && len(dispNode.Value) < 1 {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleMetadataSchemaViolation,
			File:        "module.yaml",
			Line:        dispNode.Line,
			Field:       "displayName",
			Message:     "displayName must not be empty",
			Remediation: "Review error line and correct invalid field format according to module.schema.json.",
		})
	}

	// Check version semver
	verNode := common.FindNode(node, "version")
	if verNode != nil && !semverRegex.MatchString(verNode.Value) {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleMetadataSchemaViolation,
			File:        "module.yaml",
			Line:        verNode.Line,
			Field:       "version",
			Message:     fmt.Sprintf("version %q does not conform to semantic versioning regex", verNode.Value),
			Remediation: "Review error line and correct invalid field format according to module.schema.json.",
		})
	}

	// Check summary minLength
	sumNode := common.FindNode(node, "summary")
	if sumNode != nil && len(sumNode.Value) < 1 {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleMetadataSchemaViolation,
			File:        "module.yaml",
			Line:        sumNode.Line,
			Field:       "summary",
			Message:     "summary must not be empty",
			Remediation: "Review error line and correct invalid field format according to module.schema.json.",
		})
	}

	// Check game minLength
	gameNode := common.FindNode(node, "game")
	if gameNode != nil && len(gameNode.Value) < 1 {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleMetadataSchemaViolation,
			File:        "module.yaml",
			Line:        gameNode.Line,
			Field:       "game",
			Message:     "game must not be empty",
			Remediation: "Review error line and correct invalid field format according to module.schema.json.",
		})
	}

	// Check homepage URI if present
	homeNode := common.FindNode(node, "homepage")
	if homeNode != nil && homeNode.Value != "" {
		u, err := url.ParseRequestURI(homeNode.Value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleMetadataSchemaViolation,
				File:        "module.yaml",
				Line:        homeNode.Line,
				Field:       "homepage",
				Message:     fmt.Sprintf("homepage %q is not a valid absolute URI", homeNode.Value),
				Remediation: "Review error line and correct invalid field format according to module.schema.json.",
			})
		}
	}

	// Check gameplaneMinVersion semver if present
	gpMinNode := common.FindNode(node, "gameplaneMinVersion")
	if gpMinNode != nil && gpMinNode.Value != "" && !minVerRegex.MatchString(gpMinNode.Value) {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleMetadataSchemaViolation,
			File:        "module.yaml",
			Line:        gpMinNode.Line,
			Field:       "gameplaneMinVersion",
			Message:     fmt.Sprintf("gameplaneMinVersion %q is not a valid semver", gpMinNode.Value),
			Remediation: "Review error line and correct invalid field format according to module.schema.json.",
		})
	}

	return findings
}

// validateTemplateSchema checks template.yaml against GameTemplate CRD basics.
func validateTemplateSchema(node *yaml.Node) []Finding {
	var findings []Finding

	if node == nil || node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return []Finding{
			{
				Level:       SeverityError,
				RuleID:      RuleTemplateSchemaViolation,
				File:        "template.yaml",
				Line:        1,
				Message:     "template.yaml is empty or malformed YAML",
				Remediation: "Correct the malformed YAML field matching the schema definition.",
			},
		}
	}

	rootMap := node.Content[0]
	if rootMap.Kind != yaml.MappingNode {
		return []Finding{
			{
				Level:       SeverityError,
				RuleID:      RuleTemplateSchemaViolation,
				File:        "template.yaml",
				Line:        rootMap.Line,
				Message:     "root of template.yaml must be a mapping",
				Remediation: "Correct the malformed YAML field matching the schema definition.",
			},
		}
	}

	kindNode := common.FindNode(node, "kind")
	if kindNode == nil || kindNode.Value != "GameTemplate" {
		line := 1
		if kindNode != nil {
			line = kindNode.Line
		}
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleTemplateSchemaViolation,
			File:        "template.yaml",
			Line:        line,
			Field:       "kind",
			Message:     "kind must be \"GameTemplate\"",
			Remediation: "Correct the malformed YAML field matching the schema definition.",
		})
	}

	metaNameNode := common.FindNode(node, "metadata.name")
	if metaNameNode == nil || metaNameNode.Value == "" {
		line := 1
		if metaNode := common.FindNode(node, "metadata"); metaNode != nil {
			line = metaNode.Line
		}
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleTemplateSchemaViolation,
			File:        "template.yaml",
			Line:        line,
			Field:       "metadata.name",
			Message:     "metadata.name is required in template.yaml",
			Remediation: "Correct the malformed YAML field matching the schema definition.",
		})
	}

	specNode := common.FindNode(node, "spec")
	if specNode == nil || specNode.Kind != yaml.MappingNode {
		line := 1
		if specNode != nil {
			line = specNode.Line
		}
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleTemplateSchemaViolation,
			File:        "template.yaml",
			Line:        line,
			Field:       "spec",
			Message:     "spec mapping is required in template.yaml",
			Remediation: "Correct the malformed YAML field matching the schema definition.",
		})
	} else {
		gameNode := common.FindNode(node, "spec.game")
		if gameNode == nil || gameNode.Value == "" {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleTemplateSchemaViolation,
				File:        "template.yaml",
				Line:        specNode.Line,
				Field:       "spec.game",
				Message:     "spec.game is required in template.yaml",
				Remediation: "Specify the game identifier under spec.game.",
			})
		}
		imgNode := common.FindNode(node, "spec.image")
		if imgNode == nil || imgNode.Value == "" {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleTemplateSchemaViolation,
				File:        "template.yaml",
				Line:        specNode.Line,
				Field:       "spec.image",
				Message:     "spec.image is required in template.yaml",
				Remediation: "Specify the base container image under spec.image.",
			})
		}
	}

	return findings
}
