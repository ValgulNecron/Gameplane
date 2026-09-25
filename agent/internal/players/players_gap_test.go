package players

import "testing"

// TestCompileEntryRegex_MultilineAndErrors covers CompileEntryRegex's
// success path (verifying the (?m) prefix actually anchors ^/$ per line)
// and its error path (an invalid pattern fails to compile).
func TestCompileEntryRegex_MultilineAndErrors(t *testing.T) {
	re, err := CompileEntryRegex(`^(\w+)$`)
	if err != nil {
		t.Fatalf("CompileEntryRegex: %v", err)
	}
	matches := re.FindAllStringSubmatch("alice\nbob\ncarol", -1)
	if len(matches) != 3 {
		t.Fatalf("multiline match count = %d, want 3 (got %v)", len(matches), matches)
	}

	if _, err := CompileEntryRegex("(unclosed"); err == nil {
		t.Fatal("CompileEntryRegex should error on an invalid pattern")
	}
}

// TestCompileEntryRegex_AlreadyMultiline covers the case where the
// declared pattern already carries its own (?m) flag — prepending another
// one must remain harmless rather than erroring or double-applying.
func TestCompileEntryRegex_AlreadyMultiline(t *testing.T) {
	re, err := CompileEntryRegex(`(?m)^(\w+)$`)
	if err != nil {
		t.Fatalf("CompileEntryRegex with pre-existing (?m): %v", err)
	}
	if !re.MatchString("alice") {
		t.Errorf("expected pattern to still match")
	}
}

// TestCountWithRegex covers every branch of CountWithRegex: no matches,
// a capture group whose value is counted, an empty-but-participating
// capture group that is dropped, and falling back to the whole match when
// there is no capture group at all.
func TestCountWithRegex(t *testing.T) {
	t.Run("no matches", func(t *testing.T) {
		re, _ := CompileEntryRegex(`^\[(\w+)\]$`)
		if got := CountWithRegex("nothing here", re); got != 0 {
			t.Errorf("count = %d, want 0", got)
		}
	})

	t.Run("counts capture group matches", func(t *testing.T) {
		re, _ := CompileEntryRegex(`^\[(\w+)\] joined$`)
		raw := "[alice] joined\n[bob] joined\nunrelated line\n[carol] joined"
		if got := CountWithRegex(raw, re); got != 3 {
			t.Errorf("count = %d, want 3", got)
		}
	})

	t.Run("empty capture group is dropped", func(t *testing.T) {
		// The group participates but matches an empty string on one line.
		re, _ := CompileEntryRegex(`^player:(\w*)$`)
		raw := "player:alice\nplayer:\nplayer:bob"
		if got := CountWithRegex(raw, re); got != 2 {
			t.Errorf("count = %d, want 2 (empty capture dropped)", got)
		}
	})

	t.Run("falls back to whole match without a capture group", func(t *testing.T) {
		re, _ := CompileEntryRegex(`^online$`)
		raw := "online\noffline\nonline"
		if got := CountWithRegex(raw, re); got != 2 {
			t.Errorf("count = %d, want 2", got)
		}
	})
}
