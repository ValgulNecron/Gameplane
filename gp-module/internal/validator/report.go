// Package validator provides offline static validation and linting of module manifests.
package validator

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Severity indicates finding severity level.
type Severity string

const (
	SeverityError Severity = "ERROR"
	SeverityWarn  Severity = "WARN"
)

// Rule ID constants matching contracts/diagnostics-contract.md.
const (
	RuleMissingRequiredFile     = "missing-required-file"
	RuleInvalidModuleName       = "invalid-module-name"
	RuleMetadataSchemaViolation = "metadata-schema-violation"
	RuleTemplateSchemaViolation = "template-schema-violation"
	RuleImageUnpinned           = "image-unpinned"
	RuleUnrecognizedCategory    = "unrecognized-category"
	RuleInvalidPortNumber       = "invalid-port-number"
	RuleInvalidPortProtocol     = "invalid-port-protocol"
	RuleDuplicatePortCollision  = "duplicate-port-collision"
	RuleInvalidConfigType       = "invalid-config-type"
	RuleInvalidMemoryPercent    = "invalid-memory-percent"
	RuleInsecureAuthField       = "credential-field-not-password"
	RuleExcessiveAssetSize      = "excessive-asset-size"
)

// Finding represents a single diagnostic finding.
type Finding struct {
	Level       Severity `json:"level"`
	RuleID      string   `json:"ruleId"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Field       string   `json:"field,omitempty"`
	Message     string   `json:"message"`
	Remediation string   `json:"remediation"`
}

// ModuleReport contains the validation findings for a single module.
type ModuleReport struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Clean    bool      `json:"clean"`
	Findings []Finding `json:"findings"`
}

// ValidationReport contains aggregated findings across all validated modules.
type ValidationReport struct {
	Timestamp    string         `json:"timestamp"`
	Clean        bool           `json:"clean"`
	ErrorCount   int            `json:"errorCount"`
	WarningCount int            `json:"warningCount"`
	Modules      []ModuleReport `json:"modules"`
}

// NewValidationReport creates a new ValidationReport with current timestamp.
func NewValidationReport(modules []ModuleReport) ValidationReport {
	errCount := 0
	warnCount := 0
	clean := true

	if modules == nil {
		modules = []ModuleReport{}
	}

	for i := range modules {
		if modules[i].Findings == nil {
			modules[i].Findings = []Finding{}
		}
		if !modules[i].Clean {
			clean = false
		}
		for _, f := range modules[i].Findings {
			if f.Level == SeverityError {
				errCount++
			} else if f.Level == SeverityWarn {
				warnCount++
			}
		}
	}

	return ValidationReport{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Clean:        clean,
		ErrorCount:   errCount,
		WarningCount: warnCount,
		Modules:      modules,
	}
}

// ToJSON returns indented JSON representation matching diagnostics-contract.md.
func (r *ValidationReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FormatHuman returns a human-readable terminal output.
func (r *ValidationReport) FormatHuman() string {
	var sb strings.Builder

	for _, m := range r.Modules {
		sb.WriteString(fmt.Sprintf("== %s ==\n", m.Name))
		if len(m.Findings) == 0 {
			sb.WriteString("  OK (no findings)\n")
		} else {
			for _, f := range m.Findings {
				loc := f.File
				if f.Line > 0 {
					loc = fmt.Sprintf("%s:%d", f.File, f.Line)
				}
				sb.WriteString(fmt.Sprintf("  %-5s [%s] %s: %s\n", f.Level, f.RuleID, loc, f.Message))
				if f.Remediation != "" {
					sb.WriteString(fmt.Sprintf("        Remediation: %s\n", f.Remediation))
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("SUMMARY: %d module(s) checked, %d error(s), %d warning(s).\n",
		len(r.Modules), r.ErrorCount, r.WarningCount))

	return sb.String()
}
