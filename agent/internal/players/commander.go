package players

import (
	"regexp"
	"strings"
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

func pickCommander(game string) commander {
	switch strings.ToLower(strings.TrimSpace(game)) {
	case "minecraft", "minecraft-java":
		return minecraftCommander{}
	default:
		return unsupportedCommander{}
	}
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
