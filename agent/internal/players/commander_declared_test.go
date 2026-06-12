package players

import (
	"reflect"
	"testing"

	"github.com/kestrel-gg/kestrel/agent/internal/caps"
)

// minecraftActions declares the same behavior the hardcoded minecraft
// commander implements — what modules/minecraft-java/template.yaml ships.
func minecraftActions() *caps.PlayerActions {
	return &caps.PlayerActions{
		Kick:  "kick {{.Player}}{{if .Reason}} {{.Reason}}{{end}}",
		Ban:   "ban {{.Player}}{{if .Reason}} {{.Reason}}{{end}}",
		Unban: "pardon {{.Player}}",
		BanList: &caps.BanList{
			Command:    "banlist players",
			EntryRegex: `^\s*(?P<name>[A-Za-z0-9_]{1,32})\s+was banned by\s+(?P<source>[^:]+?)(?::\s*(?P<reason>.*))?\s*$`,
		},
	}
}

func TestPickCommander_PrefersDeclared(t *testing.T) {
	// Declared actions win even for unknown games.
	c := pickCommander("factorio", minecraftActions())
	if caps := c.Capabilities(); !caps.Kick || !caps.Ban || !caps.Unban {
		t.Fatalf("capabilities = %+v", caps)
	}
	// Without declarations, only known games keep working.
	if caps := pickCommander("factorio", nil).Capabilities(); caps.Kick {
		t.Fatal("undeclared unknown game should be unsupported")
	}
	if caps := pickCommander("minecraft-java", nil).Capabilities(); !caps.Kick {
		t.Fatal("minecraft fallback lost")
	}
}

func TestTemplateCommander_MatchesHardcodedMinecraft(t *testing.T) {
	declared := pickCommander("minecraft-java", minecraftActions())
	hardcoded := minecraftCommander{}

	cases := []struct {
		name, reason string
	}{
		{"griefer", ""},
		{"griefer", "stop that"},
	}
	for _, tc := range cases {
		dk, _ := declared.Kick(tc.name, tc.reason)
		hk, _ := hardcoded.Kick(tc.name, tc.reason)
		if dk != hk {
			t.Errorf("Kick(%q,%q): declared %q != hardcoded %q", tc.name, tc.reason, dk, hk)
		}
		db, _ := declared.Ban(tc.name, tc.reason)
		hb, _ := hardcoded.Ban(tc.name, tc.reason)
		if db != hb {
			t.Errorf("Ban(%q,%q): declared %q != hardcoded %q", tc.name, tc.reason, db, hb)
		}
	}
	du, _ := declared.Unban("griefer")
	if du != "pardon griefer" {
		t.Errorf("Unban = %q", du)
	}
	dCmd, ok := declared.BanList()
	if !ok || dCmd != "banlist players" {
		t.Errorf("BanList = %q ok=%v", dCmd, ok)
	}

	raw := "griefer1 was banned by Server: Cheating\n" +
		"griefer2 was banned by alice: <no reason given>\n" +
		"griefer3 was banned by Server\n" +
		"not a ban line\n"
	if got, want := declared.ParseBanList(raw), hardcoded.ParseBanList(raw); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseBanList: declared %+v != hardcoded %+v", got, want)
	}
}

func TestTemplateCommander_PartialDeclarations(t *testing.T) {
	c := pickCommander("valheim", &caps.PlayerActions{Kick: "kick {{.Player}}"})
	if caps := c.Capabilities(); !caps.Kick || caps.Ban || caps.Unban {
		t.Fatalf("capabilities = %+v", caps)
	}
	if _, ok := c.Ban("x", ""); ok {
		t.Error("undeclared ban should be unsupported")
	}
	if _, ok := c.BanList(); ok {
		t.Error("undeclared banlist should be unsupported")
	}
	if c.ParseBanList("anything") != nil {
		t.Error("undeclared banlist parse should return nil")
	}
	cmd, ok := c.Kick("griefer", "ignored")
	if !ok || cmd != "kick griefer" {
		t.Errorf("Kick = %q ok=%v", cmd, ok)
	}
}

func TestTemplateCommander_BadDeclarationsDegrade(t *testing.T) {
	t.Run("broken template disables one action", func(t *testing.T) {
		c := pickCommander("g", &caps.PlayerActions{
			Kick: "kick {{.Player", // unparsable
			Ban:  "ban {{.Player}}",
		})
		if _, ok := c.Kick("x", ""); ok {
			t.Error("broken kick template should be unsupported")
		}
		if cmd, ok := c.Ban("x", ""); !ok || cmd != "ban x" {
			t.Errorf("ban should still work, got %q ok=%v", cmd, ok)
		}
	})

	t.Run("bad banlist regex disables banlist", func(t *testing.T) {
		c := pickCommander("g", &caps.PlayerActions{
			BanList: &caps.BanList{Command: "banlist", EntryRegex: "(unclosed"},
		})
		if _, ok := c.BanList(); ok {
			t.Error("bad regex should disable banlist")
		}
	})

	t.Run("regex without name group disables banlist", func(t *testing.T) {
		c := pickCommander("g", &caps.PlayerActions{
			BanList: &caps.BanList{Command: "banlist", EntryRegex: `^(\w+)$`},
		})
		if _, ok := c.BanList(); ok {
			t.Error("regex without name group should disable banlist")
		}
	})

	t.Run("template rendering to empty is unsupported", func(t *testing.T) {
		c := pickCommander("g", &caps.PlayerActions{Kick: "{{if .Reason}}kick {{.Player}}{{end}}"})
		if _, ok := c.Kick("x", ""); ok {
			t.Error("empty render should be unsupported")
		}
		if cmd, ok := c.Kick("x", "r"); !ok || cmd != "kick x" {
			t.Errorf("got %q ok=%v", cmd, ok)
		}
	})
}
