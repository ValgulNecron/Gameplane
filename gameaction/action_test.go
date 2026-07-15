package gameaction

import (
	"strings"
	"testing"
)

func TestResolve_Defaults(t *testing.T) {
	decls := []Param{
		{Name: "kind", Type: "enum", Enum: []string{"clear", "rain"}, Default: "clear"},
		{Name: "secs", Type: "int", Default: "60"},
		{Name: "hard", Type: "bool", Default: "false"},
	}
	got, err := Resolve(decls, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := map[string]string{"kind": "clear", "secs": "60", "hard": "false"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestResolve_RequiredMissing(t *testing.T) {
	decls := []Param{{Name: "message", Type: "string", Required: true}}
	if _, err := Resolve(decls, nil); err == nil {
		t.Fatal("Resolve: want error for missing required param")
	}
	if _, err := Resolve(decls, map[string]string{"message": "   "}); err == nil {
		t.Fatal("Resolve: want error for whitespace-only required param")
	}
}

func TestResolve_RejectsControlChars(t *testing.T) {
	decls := []Param{{Name: "message", Type: "string"}}
	_, err := Resolve(decls, map[string]string{"message": "hi\nstop"})
	if err == nil {
		t.Fatal("Resolve: want error for control character in string param")
	}
}

func TestResolve_IntBoolEnum(t *testing.T) {
	cases := []struct {
		name    string
		decl    Param
		val     string
		wantErr bool
	}{
		{"int valid", Param{Name: "secs", Type: "int"}, "42", false},
		{"int invalid", Param{Name: "secs", Type: "int"}, "soon", true},
		{"bool true", Param{Name: "hard", Type: "bool"}, "true", false},
		{"bool false", Param{Name: "hard", Type: "bool"}, "false", false},
		{"bool invalid", Param{Name: "hard", Type: "bool"}, "maybe", true},
		{"enum valid", Param{Name: "kind", Type: "enum", Enum: []string{"clear", "rain"}}, "rain", false},
		{"enum invalid", Param{Name: "kind", Type: "enum", Enum: []string{"clear", "rain"}}, "snow", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve([]Param{tc.decl}, map[string]string{tc.decl.Name: tc.val})
			if (err != nil) != tc.wantErr {
				t.Errorf("Resolve(%q) err = %v, wantErr %v", tc.val, err, tc.wantErr)
			}
		})
	}
}

func TestResolve_TooLong(t *testing.T) {
	decls := []Param{{Name: "message", Type: "string"}}
	long := strings.Repeat("a", 513)
	if _, err := Resolve(decls, map[string]string{"message": long}); err == nil {
		t.Fatal("Resolve: want error for a 513-char string param")
	}
	ok := strings.Repeat("a", 512)
	if _, err := Resolve(decls, map[string]string{"message": ok}); err != nil {
		t.Errorf("Resolve: unexpected error for a 512-char string param: %v", err)
	}
}

func TestCompile_BadTemplate(t *testing.T) {
	if _, err := Compile("broken", "say {{.Params"); err == nil {
		t.Fatal("Compile: want error for malformed template")
	}
}

func TestRender_Params(t *testing.T) {
	cmd, err := Compile("broadcast", "say {{.Params.message}}")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	got, err := cmd.Render(map[string]string{"message": "hi"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "say hi" {
		t.Errorf("Render = %q, want %q", got, "say hi")
	}
}

func TestRender_MissingKey(t *testing.T) {
	cmd, err := Compile("broken", "say {{.Params.nope}}")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, err := cmd.Render(map[string]string{"message": "hi"}); err == nil {
		t.Fatal("Render: want error referencing an undeclared key")
	}
}
