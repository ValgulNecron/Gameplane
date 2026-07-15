// Package gameaction holds the console-injection guard and command-template
// renderer shared by every transport that runs a module-declared action
// against a game: the agent (RCON) and the API (stdin pod-attach). Both
// importers call Resolve before rendering — each is its own trust boundary,
// so validation is never skipped because "the other side already checked."
package gameaction

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// Param is a declared action input. Mirrors the CRD's ActionParamSpec and
// the agent's caps.ActionParam — one canonical shape both importers share.
type Param struct {
	Name        string
	DisplayName string
	Description string
	Type        string // "string" | "int" | "bool" | "enum"; "" == string
	Default     string
	Enum        []string
	Required    bool
}

// Resolve validates raw user values against the declared params and returns
// the sanitized map. It rejects control characters in string params (the
// console-injection guard), enforces types, the 512-char cap, enum
// membership, and required-ness.
func Resolve(decls []Param, got map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(decls))
	for _, p := range decls {
		val, ok := got[p.Name]
		if !ok || val == "" {
			val = p.Default
		}
		if p.Required && strings.TrimSpace(val) == "" {
			return nil, fmt.Errorf("parameter %q is required", p.Name)
		}
		if val == "" {
			out[p.Name] = ""
			continue
		}
		clean, err := validateParam(p, val)
		if err != nil {
			return nil, err
		}
		out[p.Name] = clean
	}
	return out, nil
}

func validateParam(p Param, val string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(p.Type)) {
	case "int":
		if _, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err != nil {
			return "", fmt.Errorf("parameter %q must be an integer", p.Name)
		}
		return strings.TrimSpace(val), nil
	case "bool":
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true", "false":
			return strings.ToLower(strings.TrimSpace(val)), nil
		}
		return "", fmt.Errorf("parameter %q must be true or false", p.Name)
	case "enum":
		for _, e := range p.Enum {
			if val == e {
				return val, nil
			}
		}
		return "", fmt.Errorf("parameter %q must be one of the declared options", p.Name)
	default:
		if hasControl(val) {
			return "", fmt.Errorf("parameter %q must not contain control characters", p.Name)
		}
		if len(val) > 512 {
			return "", fmt.Errorf("parameter %q is too long (max 512)", p.Name)
		}
		return val, nil
	}
}

// hasControl reports whether s contains an ASCII control character.
// Rejecting these (notably CR/LF) stops a parameter value from chaining a
// second console command into the rendered line.
func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// Command is a parsed action command template.
type Command struct {
	tmpl *template.Template
}

// Compile parses one command template (missingkey=error). name is only for
// error messages.
func Compile(name, command string) (*Command, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(command)
	if err != nil {
		return nil, fmt.Errorf("parse action command %q: %w", name, err)
	}
	return &Command{tmpl: t}, nil
}

// Render executes the template with the resolved params, returning the
// trimmed command string. Params are exposed to the template as .Params.
func (c *Command) Render(params map[string]string) (string, error) {
	var sb strings.Builder
	if err := c.tmpl.Execute(&sb, struct{ Params map[string]string }{params}); err != nil {
		return "", fmt.Errorf("render action command: %w", err)
	}
	return strings.TrimSpace(sb.String()), nil
}
