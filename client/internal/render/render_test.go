package render

// In-package unit tests for the two pure helpers every card goes through:
// clean must strip every byte a terminal would act on, and relative must keep
// the human boundaries of the freshness line.

import (
	"testing"
	"time"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		give string
		want string
	}{
		{
			name: "plain text passes through",
			give: "Fix parser & renderer",
			want: "Fix parser & renderer",
		},
		{
			name: "empty string stays empty",
			give: "",
			want: "",
		},
		{
			name: "esc sequence loses only the esc byte",
			give: "\x1b[2K",
			want: "[2K",
		},
		{
			name: "newline becomes a space",
			give: "two\nlines",
			want: "two lines",
		},
		{
			name: "tab becomes a space",
			give: "two\ttabs",
			want: "two tabs",
		},
		{
			name: "carriage return is dropped, not spaced",
			give: "carriage\rreturn",
			want: "carriagereturn",
		},
		{
			name: "bell is dropped",
			give: "ring\x07",
			want: "ring",
		},
		{
			name: "del is dropped",
			give: "erase\x7f",
			want: "erase",
		},
		{
			// The escapes name the C1 code points directly; a \x85 byte here
			// would be invalid UTF-8, which clean keeps as U+FFFD instead.
			name: "c1 control code points are dropped",
			give: "nel" + string(rune(0x85)) + "csi" + string(rune(0x9b)) + "end",
			want: "nelcsiend",
		},
		{
			name: "printable multibyte text survives",
			give: "señal ñandú",
			want: "señal ñandú",
		},
		{
			// The hostile subject client/test pins: without its esc bytes the
			// OSC 8 sequence can no longer retarget the hyperlink.
			name: "hostile hyperlink loses its esc bytes",
			give: "Tidy \x1b]8;;https://evil.example/\x1b\\ now",
			want: "Tidy ]8;;https://evil.example/\\ now",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clean(tt.give); got != tt.want {
				t.Fatalf("clean(%q) = %q, want %q", tt.give, got, tt.want)
			}
		})
	}
}

func TestRelative(t *testing.T) {
	now := time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		ago  time.Duration
		want string
	}{
		{name: "seconds ago is just now", ago: 30 * time.Second, want: "just now"},
		{name: "exactly one minute starts the minutes", ago: time.Minute, want: "1m ago"},
		{name: "minutes truncate", ago: 5*time.Minute + 30*time.Second, want: "5m ago"},
		{name: "exactly one hour starts the hours", ago: time.Hour, want: "1h ago"},
		{name: "hours truncate", ago: 3*time.Hour + 59*time.Minute, want: "3h ago"},
		{name: "twenty-three hours", ago: 23 * time.Hour, want: "23h ago"},
		{name: "exactly one day is yesterday", ago: 24 * time.Hour, want: "yesterday"},
		{name: "the 24-to-48 hour window is yesterday", ago: 25 * time.Hour, want: "yesterday"},
		{name: "just under two days", ago: 47*time.Hour + 59*time.Minute, want: "yesterday"},
		{name: "exactly two days counts in days", ago: 48 * time.Hour, want: "2d ago"},
		{name: "a week", ago: 7 * 24 * time.Hour, want: "7d ago"},
		{name: "a future event reads as just now", ago: -time.Hour, want: "just now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			when := now.Add(-tt.ago)
			if got := relative(now, when); got != tt.want {
				t.Fatalf("relative(%v, %v) = %q, want %q", now, when, got, tt.want)
			}
		})
	}
}
