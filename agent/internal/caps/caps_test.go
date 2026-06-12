package caps

import "testing"

func TestParse(t *testing.T) {
	t.Run("empty means none", func(t *testing.T) {
		s, err := Parse("")
		if s != nil || err != nil {
			t.Fatalf("s=%v err=%v", s, err)
		}
	})

	t.Run("full document", func(t *testing.T) {
		s, err := Parse(`{
			"players": {
				"kick": "kick {{.Player}}",
				"banList": {"command": "banlist players", "entryRegex": "^(?P<name>\\w+)$"}
			},
			"quiesce": {"quiesce": ["save-off"], "unquiesce": ["save-on"], "failurePattern": "failed"}
		}`)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if s.Players == nil || s.Players.Kick != "kick {{.Player}}" || s.Players.BanList == nil {
			t.Errorf("players = %+v", s.Players)
		}
		if s.Quiesce == nil || len(s.Quiesce.Quiesce) != 1 || s.Quiesce.FailurePattern != "failed" {
			t.Errorf("quiesce = %+v", s.Quiesce)
		}
	})

	t.Run("malformed json errors", func(t *testing.T) {
		if _, err := Parse("{not json"); err == nil {
			t.Fatal("expected error")
		}
	})
}
