package cli

import (
	"flag"
	"testing"
)

// TestRefused pins the refusal wording byte for byte: the security suite
// asserts "answered 302" on stderr, and both read paths must produce the same
// bytes from the same helper.
func TestRefused(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		status string
		want   string
	}{
		{"names the relay and the status it refused",
			"http://localhost:3000/history", "302 Found",
			"http://localhost:3000/history answered 302 Found"},
		{"spells out the control bytes in the status",
			"http://localhost:3000/stream", "500 \x1b]0;pwned\x07",
			"http://localhost:3000/stream answered 500 \\x1b]0;pwned\\x07"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := refused(tt.url, tt.status)
			if err == nil {
				t.Fatal("refused() = nil, want an error")
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("refused() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPrintable pins the stderr contract: relay-authored text keeps its shape
// while the bytes a terminal would act on are spelled out instead.
func TestPrintable(t *testing.T) {
	tests := []struct {
		name string
		give string
		want string
	}{
		{"a plain status line passes through", "404 Not Found", "404 Not Found"},
		{"escape and bell are spelled out", "\x1b]0;pwned\x07status\x1b[2J",
			"\\x1b]0;pwned\\x07status\\x1b[2J"},
		{"nul and delete are spelled out", "500 \x00\x7f", "500 \\x00\\x7f"},
		{"a c1 control rune is spelled out", "500 " + string(rune(0x85)), "500 \\x85"},
		{"multibyte text passes through", "500 héllo ✓", "500 héllo ✓"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printable(tt.give); got != tt.want {
				t.Errorf("printable(%q) = %q, want %q", tt.give, got, tt.want)
			}
		})
	}
}

// TestAddFilters pins the one registration point: both commands register the
// flags under the same names, defaults, and help text, and nothing else.
func TestAddFilters(t *testing.T) {
	fs := flag.NewFlagSet("filters", flag.ContinueOnError)
	user, repository := addFilters(fs)

	tests := []struct {
		name  string
		usage string
	}{
		{"user", "only events where this login is the actor or the assignee"},
		{"repo", "only events in this owner/name repository"},
	}
	for _, tt := range tests {
		f := fs.Lookup(tt.name)
		if f == nil {
			t.Fatalf("flag %q not registered by addFilters", tt.name)
		}
		if f.DefValue != "" {
			t.Errorf("flag %q default = %q, want the empty default", tt.name, f.DefValue)
		}
		if f.Usage != tt.usage {
			t.Errorf("flag %q help = %q, want %q", tt.name, f.Usage, tt.usage)
		}
	}
	if *user != "" || *repository != "" {
		t.Errorf("initial values = (%q, %q), want both empty before parsing", *user, *repository)
	}

	registered := 0
	fs.VisitAll(func(*flag.Flag) { registered++ })
	if registered != len(tests) {
		t.Errorf("registered flags = %d, want exactly the %d filters", registered, len(tests))
	}

	if err := fs.Parse([]string{"--user", "octocat", "--repo", "octo/widget"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if *user != "octocat" {
		t.Errorf("user after parse = %q, want %q", *user, "octocat")
	}
	if *repository != "octo/widget" {
		t.Errorf("repository after parse = %q, want %q", *repository, "octo/widget")
	}
}
