package controller

import (
	"testing"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

func TestEffectiveConsoleMode(t *testing.T) {
	cases := []struct {
		name string
		spec kestrelv1alpha1.GameTemplateSpec
		want string
	}{
		{
			name: "explicit pty wins over rcon spec",
			spec: kestrelv1alpha1.GameTemplateSpec{
				ConsoleMode: "pty",
				RCON:        &kestrelv1alpha1.RCONSpec{Protocol: "source"},
			},
			want: "pty",
		},
		{
			name: "explicit none wins over rcon spec",
			spec: kestrelv1alpha1.GameTemplateSpec{
				ConsoleMode: "none",
				RCON:        &kestrelv1alpha1.RCONSpec{Protocol: "source"},
			},
			want: "none",
		},
		{
			name: "explicit rcon",
			spec: kestrelv1alpha1.GameTemplateSpec{ConsoleMode: "rcon"},
			want: "rcon",
		},
		{
			name: "default with rcon source → rcon",
			spec: kestrelv1alpha1.GameTemplateSpec{
				RCON: &kestrelv1alpha1.RCONSpec{Protocol: "source"},
			},
			want: "rcon",
		},
		{
			name: "default with rcon protocol=none → none",
			spec: kestrelv1alpha1.GameTemplateSpec{
				RCON: &kestrelv1alpha1.RCONSpec{Protocol: "none"},
			},
			want: "none",
		},
		{
			name: "default with no rcon at all → none",
			spec: kestrelv1alpha1.GameTemplateSpec{},
			want: "none",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveConsoleMode(&kestrelv1alpha1.GameTemplate{Spec: tc.spec})
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}

	if got := EffectiveConsoleMode(nil); got != "none" {
		t.Errorf("nil template: got %q want \"none\"", got)
	}
}
