package players

import (
	"log/slog"
	"regexp"
	"strings"
	"text/template"

	"github.com/kestrel-gg/kestrel/agent/internal/caps"
)

// commander formats per-game RCON commands for moderation actions.
// Methods return (cmd, ok=false) when the game doesn't support that
// action — the handler turns ok=false into a 501 response.
type commander interface {
	Capabilities() Capabilities
	Kick(name, reason string) (string, bool)
	Ban(name, reason string) (string, bool)
	Unban(name string) (string, bool)
	BanList() (string, bool)
	ParseBanList(raw string) []BannedPlayer
}

// pickCommander prefers commands declared by the module's template
// (spec.capabilities.players). The hardcoded minecraft fallback covers
// GameTemplates materialized before capabilities existed; it goes away
// once the bundled modules all declare their commands.
func pickCommander(game string, actions *caps.PlayerActions) commander {
	if actions != nil {
		return newTemplateCommander(actions)
	}
	switch strings.ToLower(strings.TrimSpace(game)) {
	case "minecraft", "minecraft-java":
		return minecraftCommander{}
	default:
		return unsupportedCommander{}
	}
}

// --- Declared (template-driven) commanders --------------------------------

// modVars is the render context for action command templates.
type modVars struct {
	Player string
	Reason string
}

type templateCommander struct {
	kick, ban, unban *template.Template
	banListCmd       string
	banListRE        *regexp.Regexp
}

// newTemplateCommander compiles the declared action templates. A
// malformed template or regex disables that single action (logged) —
// one bad declaration must not take down the whole moderation surface.
func newTemplateCommander(actions *caps.PlayerActions) commander {
	parse := func(action, text string) *template.Template {
		if text == "" {
			return nil
		}
		t, err := template.New(action).Parse(text)
		if err != nil {
			slog.Warn("invalid capability command template; action disabled",
				"action", action, "err", err)
			return nil
		}
		return t
	}
	c := templateCommander{
		kick:  parse("kick", actions.Kick),
		ban:   parse("ban", actions.Ban),
		unban: parse("unban", actions.Unban),
	}
	if bl := actions.BanList; bl != nil && bl.Command != "" {
		re, err := regexp.Compile(bl.EntryRegex)
		switch {
		case err != nil:
			slog.Warn("invalid banList entryRegex; ban list disabled", "err", err)
		case re.SubexpIndex("name") < 0:
			slog.Warn("banList entryRegex has no (?P<name>…) group; ban list disabled")
		default:
			c.banListCmd, c.banListRE = bl.Command, re
		}
	}
	return c
}

func (c templateCommander) Capabilities() Capabilities {
	return Capabilities{Kick: c.kick != nil, Ban: c.ban != nil, Unban: c.unban != nil}
}

func (c templateCommander) render(t *template.Template, name, reason string) (string, bool) {
	if t == nil {
		return "", false
	}
	var sb strings.Builder
	if err := t.Execute(&sb, modVars{Player: name, Reason: reason}); err != nil {
		slog.Warn("render capability command", "template", t.Name(), "err", err)
		return "", false
	}
	cmd := strings.TrimSpace(sb.String())
	if cmd == "" {
		return "", false
	}
	return cmd, true
}

func (c templateCommander) Kick(name, reason string) (string, bool) {
	return c.render(c.kick, name, reason)
}

func (c templateCommander) Ban(name, reason string) (string, bool) {
	return c.render(c.ban, name, reason)
}

func (c templateCommander) Unban(name string) (string, bool) {
	return c.render(c.unban, name, "")
}

func (c templateCommander) BanList() (string, bool) {
	return c.banListCmd, c.banListCmd != ""
}

func (c templateCommander) ParseBanList(raw string) []BannedPlayer {
	if c.banListRE == nil {
		return nil
	}
	nameIdx := c.banListRE.SubexpIndex("name")
	sourceIdx := c.banListRE.SubexpIndex("source")
	reasonIdx := c.banListRE.SubexpIndex("reason")
	out := []BannedPlayer{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n") {
		m := c.banListRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entry := BannedPlayer{Name: m[nameIdx]}
		if sourceIdx >= 0 {
			entry.Source = strings.TrimSpace(m[sourceIdx])
		}
		if reasonIdx >= 0 {
			if r := strings.TrimSpace(m[reasonIdx]); r != "" && r != "<no reason given>" {
				entry.Reason = r
			}
		}
		out = append(out, entry)
	}
	return out
}

// --- Minecraft (vanilla / paper / spigot / forge / fabric) ---------------

type minecraftCommander struct{}

func (minecraftCommander) Capabilities() Capabilities {
	return Capabilities{Kick: true, Ban: true, Unban: true}
}

func (minecraftCommander) Kick(name, reason string) (string, bool) {
	if reason == "" {
		return "kick " + name, true
	}
	return "kick " + name + " " + reason, true
}

func (minecraftCommander) Ban(name, reason string) (string, bool) {
	if reason == "" {
		return "ban " + name, true
	}
	return "ban " + name + " " + reason, true
}

func (minecraftCommander) Unban(name string) (string, bool) {
	return "pardon " + name, true
}

func (minecraftCommander) BanList() (string, bool) {
	return "banlist players", true
}

// banlistEntryRE matches lines of the form
//
//	griefer1 was banned by Server: Cheating
//	griefer2 was banned by alice: <no reason given>
//
// Older servers omit the reason segment entirely:
//
//	griefer3 was banned by Server
var banlistEntryRE = regexp.MustCompile(
	`^\s*([A-Za-z0-9_]{1,32})\s+was banned by\s+([^:]+?)(?::\s*(.*))?\s*$`,
)

func (minecraftCommander) ParseBanList(raw string) []BannedPlayer {
	out := []BannedPlayer{}
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n") {
		m := banlistEntryRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entry := BannedPlayer{Name: m[1], Source: strings.TrimSpace(m[2])}
		if len(m) > 3 {
			r := strings.TrimSpace(m[3])
			if r != "" && r != "<no reason given>" {
				entry.Reason = r
			}
		}
		out = append(out, entry)
	}
	return out
}

// --- Default (RCON-less or untyped games) --------------------------------

type unsupportedCommander struct{}

func (unsupportedCommander) Capabilities() Capabilities         { return Capabilities{} }
func (unsupportedCommander) Kick(string, string) (string, bool) { return "", false }
func (unsupportedCommander) Ban(string, string) (string, bool)  { return "", false }
func (unsupportedCommander) Unban(string) (string, bool)        { return "", false }
func (unsupportedCommander) BanList() (string, bool)            { return "", false }
func (unsupportedCommander) ParseBanList(string) []BannedPlayer { return nil }
