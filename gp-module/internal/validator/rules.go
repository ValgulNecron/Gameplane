// Package validator provides offline static validation and linting of module manifests.
package validator

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ValgulNecron/gameplane/gp-module/internal/common"
)

var allowedConfigTypes = map[string]bool{
	"string":   true,
	"int":      true,
	"bool":     true,
	"boolean":  true,
	"enum":     true,
	"password": true,
}

var credentialKeywords = []string{
	"PASSWORD",
	"TOKEN",
	"SECRET",
	"KEY",
	"AUTH",
}

func validateModuleName(name string, dirName string, node *yaml.Node) []Finding {
	var findings []Finding

	nameNode := common.FindNode(node, "name")
	line := 1
	if nameNode != nil {
		line = nameNode.Line
	}

	if err := common.ValidateModuleName(name); err != nil {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleInvalidModuleName,
			File:        "module.yaml",
			Line:        line,
			Field:       "name",
			Message:     fmt.Sprintf("module name %q is invalid: %v", name, err),
			Remediation: "Rename the directory or update 'name' in module.yaml to match lower-case alphanumeric DNS-1123 format.",
		})
	} else if dirName != "" && dirName != "." && dirName != name {
		findings = append(findings, Finding{
			Level:       SeverityError,
			RuleID:      RuleInvalidModuleName,
			File:        "module.yaml",
			Line:        line,
			Field:       "name",
			Message:     fmt.Sprintf("module name %q does not match directory name %q", name, dirName),
			Remediation: "Rename the directory or update 'name' in module.yaml to match lower-case alphanumeric DNS-1123 format.",
		})
	}

	return findings
}

func validateCategories(node *yaml.Node) []Finding {
	var findings []Finding
	catNode := common.FindNode(node, "categories")
	if catNode == nil || catNode.Kind != yaml.SequenceNode {
		return nil
	}

	for _, item := range catNode.Content {
		if !common.IsCanonicalCategory(item.Value) {
			findings = append(findings, Finding{
				Level:       SeverityWarn,
				RuleID:      RuleUnrecognizedCategory,
				File:        "module.yaml",
				Line:        item.Line,
				Field:       "categories",
				Message:     fmt.Sprintf("category %q is not in the canonical Gameplane taxonomy", item.Value),
				Remediation: "Use a canonical category where possible to align with catalog filter chips.",
			})
		}
	}

	return findings
}

func validateImageDigest(node *yaml.Node, rawContent string) []Finding {
	var findings []Finding
	imgNode := common.FindNode(node, "spec.image")
	if imgNode == nil {
		return nil
	}

	val := imgNode.Value
	if strings.Contains(val, "@sha256:") {
		// Image is pinned
		return nil
	}

	// Check for floating opt-out comment
	if strings.Contains(imgNode.LineComment, "gameplane:floating") ||
		strings.Contains(imgNode.HeadComment, "gameplane:floating") {
		return nil
	}

	// Also check raw line in case comment wasn't attached to the node
	lines := strings.Split(rawContent, "\n")
	if imgNode.Line > 0 && imgNode.Line <= len(lines) {
		lineStr := lines[imgNode.Line-1]
		if strings.Contains(lineStr, "gameplane:floating") {
			return nil
		}
	}

	findings = append(findings, Finding{
		Level:       SeverityError,
		RuleID:      RuleImageUnpinned,
		File:        "template.yaml",
		Line:        imgNode.Line,
		Field:       "spec.image",
		Message:     fmt.Sprintf("default image %q is not pinned with @sha256 digest", val),
		Remediation: "Pin image digest using 'gp-module pin' / 'make module-pin' or append '# gameplane:floating' if intentionally dynamic.",
	})

	return findings
}

func validatePorts(node *yaml.Node) []Finding {
	var findings []Finding
	portsNode := common.FindNode(node, "spec.ports")
	if portsNode == nil || portsNode.Kind != yaml.SequenceNode {
		return nil
	}

	type portKey struct {
		port  int
		proto string
	}
	seen := make(map[portKey]int)

	for i, pNode := range portsNode.Content {
		if pNode.Kind != yaml.MappingNode {
			continue
		}

		portField := common.FindNode(pNode, "containerPort")
		protoField := common.FindNode(pNode, "protocol")

		proto := "TCP"
		protoLine := pNode.Line
		if protoField != nil {
			proto = protoField.Value
			protoLine = protoField.Line
		}

		if proto != "TCP" && proto != "UDP" {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleInvalidPortProtocol,
				File:        "template.yaml",
				Line:        protoLine,
				Field:       fmt.Sprintf("spec.ports[%d].protocol", i),
				Message:     fmt.Sprintf("protocol %q is invalid (must be TCP or UDP)", proto),
				Remediation: "Change protocol to either TCP or UDP.",
			})
		}

		if portField == nil {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleInvalidPortNumber,
				File:        "template.yaml",
				Line:        pNode.Line,
				Field:       fmt.Sprintf("spec.ports[%d].containerPort", i),
				Message:     "containerPort is required",
				Remediation: "Change containerPort to an integer between 1 and 65535.",
			})
			continue
		}

		portNum, err := strconv.Atoi(portField.Value)
		if err != nil || portNum < 1 || portNum > 65535 {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleInvalidPortNumber,
				File:        "template.yaml",
				Line:        portField.Line,
				Field:       fmt.Sprintf("spec.ports[%d].containerPort", i),
				Message:     fmt.Sprintf("containerPort %q is outside allowable range 1-65535", portField.Value),
				Remediation: "Change containerPort to an integer between 1 and 65535.",
			})
			continue
		}

		key := portKey{port: portNum, proto: proto}
		if prevLine, ok := seen[key]; ok {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleDuplicatePortCollision,
				File:        "template.yaml",
				Line:        portField.Line,
				Field:       fmt.Sprintf("spec.ports[%d]", i),
				Message:     fmt.Sprintf("duplicate port declaration %d/%s (already declared on line %d)", portNum, proto, prevLine),
				Remediation: "Assign unique port numbers or change protocol.",
			})
		} else {
			seen[key] = portField.Line
		}
	}

	return findings
}

func validateConfigSchemaRules(node *yaml.Node) []Finding {
	var findings []Finding
	cfgNode := common.FindNode(node, "spec.configSchema")
	if cfgNode == nil || cfgNode.Kind != yaml.SequenceNode {
		return nil
	}

	for i, item := range cfgNode.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}

		nameNode := common.FindNode(item, "name")
		typeNode := common.FindNode(item, "type")

		fieldName := ""
		nameLine := item.Line
		if nameNode != nil {
			fieldName = nameNode.Value
			nameLine = nameNode.Line
		}

		fieldType := ""
		typeLine := item.Line
		if typeNode != nil {
			fieldType = typeNode.Value
			typeLine = typeNode.Line
		}

		if fieldType != "" && !allowedConfigTypes[fieldType] {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleInvalidConfigType,
				File:        "template.yaml",
				Line:        typeLine,
				Field:       fmt.Sprintf("spec.configSchema[%d].type", i),
				Message:     fmt.Sprintf("configSchema type %q is invalid (allowed: string, int, enum, boolean, password)", fieldType),
				Remediation: "Change field type to one of the supported Gameplane types.",
			})
		}

		// Credential name check
		upper := strings.ToUpper(fieldName)
		isCred := false
		for _, kw := range credentialKeywords {
			if strings.Contains(upper, kw) {
				isCred = true
				break
			}
		}

		if isCred && fieldType != "password" && fieldType != "" {
			findings = append(findings, Finding{
				Level:       SeverityError,
				RuleID:      RuleInsecureAuthField,
				File:        "template.yaml",
				Line:        nameLine,
				Field:       fmt.Sprintf("spec.configSchema[%d].name", i),
				Message:     fmt.Sprintf("field %q appears to be a credential but has type %q instead of \"password\"", fieldName, fieldType),
				Remediation: "Set 'type: password' so the operator stores value securely in a Secret instead of plaintext CR.",
			})
		}

		// autoFromMemoryLimit percent check
		memNode := common.FindNode(item, "autoFromMemoryLimit")
		if memNode != nil {
			pctNode := common.FindNode(memNode, "percent")
			if pctNode != nil {
				pctVal, err := strconv.Atoi(pctNode.Value)
				if err != nil || pctVal < 1 || pctVal > 100 {
					findings = append(findings, Finding{
						Level:       SeverityError,
						RuleID:      RuleInvalidMemoryPercent,
						File:        "template.yaml",
						Line:        pctNode.Line,
						Field:       fmt.Sprintf("spec.configSchema[%d].autoFromMemoryLimit.percent", i),
						Message:     fmt.Sprintf("autoFromMemoryLimit percent %q must be an integer between 1 and 100", pctNode.Value),
						Remediation: "Set percent between 1 and 100 (e.g. 75 for 75%).",
					})
				}
			}
		}
	}

	return findings
}

func validateAssetSizes(iconSize int64, totalSize int64) []Finding {
	var findings []Finding

	const maxIconSize = 512 * 1024    // 512 KiB
	const maxBundleSize = 1024 * 1024 // 1 MiB

	if iconSize > maxIconSize {
		findings = append(findings, Finding{
			Level:       SeverityWarn,
			RuleID:      RuleExcessiveAssetSize,
			File:        "icon.png",
			Line:        0,
			Message:     fmt.Sprintf("icon asset size (%d bytes) exceeds recommended 512 KiB limit", iconSize),
			Remediation: "Compress asset or reduce resolution to optimize OCI bundle transfer.",
		})
	}

	if totalSize > maxBundleSize {
		findings = append(findings, Finding{
			Level:       SeverityWarn,
			RuleID:      RuleExcessiveAssetSize,
			File:        "module directory",
			Line:        0,
			Message:     fmt.Sprintf("total module size (%d bytes) exceeds recommended 1 MiB limit", totalSize),
			Remediation: "Compress asset or reduce resolution to optimize OCI bundle transfer.",
		})
	}

	return findings
}
