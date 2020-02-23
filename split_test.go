package girc

import (
	"testing"
)

func TestPRIVMSG(t *testing.T) {
	type TestCase struct {
		Event   *Event
		MaxLen  int // Text length which shall not be exceeded (maxTextLen)
		Results []string
	}

	const target = "#foo"
	ev := func(text string) *Event {
		return &Event{Command: PRIVMSG, Params: []string{target, text}}
	}

	tests := []TestCase{
		{ev("foo bar baz"), 4, []string{"foo ", "bar ", "baz"}},
		{ev("1234567890"), 5, []string{"12345", "67890"}},
		{ev("unsplitted"), 10, []string{"unsplitted"}},
		{ev("ξ_λχ"), len("λχ"), []string{"ξ_", "λχ"}},
		{ev("foobar"), 0, []string{"foobar"}},
	}

	// Keep this in sync with the maxTextLen calculation in split.go
	rawEvent := &Event{Command: PRIVMSG, Params: []string{target}}
	off := rawEvent.Len() + len(" :")

	for _, test := range tests {
		events := splitPRIVMSG(test.Event, test.MaxLen+off)
		if len(events) != len(test.Results) {
			t.Fatalf("Expected %d events - got %d\n", len(test.Results), len(events))
		}

		for n, exp := range test.Results {
			if events[n].Last() != exp {
				t.Fatalf("Expected %q - got %q\n", exp, events[n].Last())
			}
		}
	}
}
