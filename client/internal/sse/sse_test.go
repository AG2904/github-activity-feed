package sse

// In-package unit tests for the frame reader: Next must yield the data payload
// of each frame, skip everything the client does not act on, and end the stream
// with io.EOF.

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name string
		give string
		// want is every data payload Next must yield, in order, before the
		// clean io.EOF.
		want []string
	}{
		{
			name: "single data frame dispatched at the blank line",
			give: "data: hello\n\n",
			want: []string{"hello"},
		},
		{
			name: "id event and retry lines are skipped",
			give: "id: 1\nevent: pr.merged\nretry: 1500\ndata: {\"type\":\"pr.merged\"}\n\n",
			want: []string{`{"type":"pr.merged"}`},
		},
		{
			name: "heartbeat comment frames are skipped",
			give: ": heartbeat\n\ndata: after the heartbeat\n\n",
			want: []string{"after the heartbeat"},
		},
		{
			name: "frame without data dispatches nothing",
			give: "id: 1\n\n",
			want: nil,
		},
		{
			name: "event-only frame at eof dispatches nothing",
			give: "event: pr.merged\n",
			want: nil,
		},
		{
			name: "multiline data joined with a newline",
			give: "data: first\ndata: second\n\n",
			want: []string{"first\nsecond"},
		},
		{
			name: "colon inside the value survives the cut",
			give: "data: {\"a\":\"b\"}\n\n",
			want: []string{`{"a":"b"}`},
		},
		{
			name: "value with no space after the colon keeps no space",
			give: "data:tight\n\n",
			want: []string{"tight"},
		},
		{
			name: "only one leading space is trimmed",
			give: "data:  spaced\n\n",
			want: []string{" spaced"},
		},
		{
			name: "crlf line endings are stripped",
			give: "data: hello\r\n\r\n",
			want: []string{"hello"},
		},
		{
			name: "frame at eof without its blank line still counts",
			give: "data: tail",
			want: []string{"tail"},
		},
		{
			name: "empty input is a clean eof",
			give: "",
			want: nil,
		},
		{
			name: "blank lines without data dispatch nothing",
			give: "\n\n",
			want: nil,
		},
		{
			name: "multiple frames yield in order",
			give: "data: one\n\ndata: two\n\n",
			want: []string{"one", "two"},
		},
		{
			name: "junk frame shapes are skipped until the data",
			give: "id: 1\n" +
				": heartbeat\n\n" +
				"event: pr.merged\n\n" +
				"retry: 1500\n\n" +
				"data: this is not json\n\n",
			want: []string{"this is not json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tt.give))
			var got []string
			for {
				line, err := r.Next()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						t.Fatalf("Next() on %q returned error %v, want io.EOF", tt.give, err)
					}
					break
				}
				got = append(got, line)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Next() on %q yielded %q, want %q", tt.give, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("Next() on %q yielded %q, want %q", tt.give, got, tt.want)
				}
			}
		})
	}
}

func TestNextReturnsTheUnderlyingReaderError(t *testing.T) {
	sentinel := errors.New("relay connection reset")
	r := NewReader(iotest.ErrReader(sentinel))

	if _, err := r.Next(); !errors.Is(err, sentinel) {
		t.Fatalf("Next() error = %v, want the reader's own error %v", err, sentinel)
	}
}

func TestNextDeliversDataSeenBeforeTheReaderError(t *testing.T) {
	sentinel := errors.New("relay connection reset")
	r := NewReader(io.MultiReader(strings.NewReader("data: partial"), iotest.ErrReader(sentinel)))

	line, err := r.Next()
	if err != nil || line != "partial" {
		t.Fatalf("Next() = (%q, %v), want (%q, nil)", line, err, "partial")
	}
	if _, err := r.Next(); !errors.Is(err, sentinel) {
		t.Fatalf("Next() after the frame error = %v, want the reader's own error %v", err, sentinel)
	}
}

func FuzzNext(f *testing.F) {
	// Seeds: the happy paths plus the junk frame shapes the e2e suite pins
	// (client/test/e2e_test.go TestWatchDiscardsFramesItCannotUse…) and
	// relay-supplied control bytes.
	seeds := []string{
		"",                              // empty stream
		"data: hello\n\n",               // the happy frame
		"data: first\ndata: second\n\n", // multiline data
		": heartbeat\n\n",               // the relay's heartbeat carries no data
		"id: 1\n" +
			": heartbeat\n\n" +
			"event: pr.merged\n\n" +
			"retry: 1500\n\n" +
			"data: {\"type\":\"pr.merged\"\n\n" +
			"data: this is not json\n\n", // junk frame shapes from the e2e suite
		"data: hello\r\n\r\n",        // crlf line endings
		"data: tail",                 // frame cut short by eof
		"data:\n\n",                  // empty value
		"data: \x1b[2K\r\x1b[1A\n\n", // control bytes inside a value
		"\x00\xff\xfe",               // raw junk bytes
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		r := NewReader(bytes.NewReader(input))
		eof := false
		for {
			line, err := r.Next()
			if err == nil {
				if eof {
					t.Fatalf("Next() yielded %q after io.EOF on input %q", line, input)
				}
				continue
			}
			if !errors.Is(err, io.EOF) {
				t.Fatalf("Next() error = %v on input %q, want nil or io.EOF", err, input)
			}
			// io.EOF must stick: the stream is over for good.
			if line, err := r.Next(); !errors.Is(err, io.EOF) || line != "" {
				t.Fatalf("Next() after io.EOF = (%q, %v), want (\"\", io.EOF) on input %q", line, err, input)
			}
			return
		}
	})
}
