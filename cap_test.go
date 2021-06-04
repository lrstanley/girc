// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"reflect"
	"testing"
)

func TestCapSupported(t *testing.T) {
	c := New(Config{
		Server:        "irc.example.com",
		Nick:          "test",
		User:          "user",
		SASL:          &SASLPlain{User: "test", Pass: "example"},
		SupportedCaps: map[string][]string{"example": nil},
	})

	var ok bool
	ls := possibleCapList(c)
	if _, ok = ls["batch"]; !ok {
		t.Fatal("possibleCapList() missing std cap")
	}

	if _, ok = ls["example"]; !ok {
		t.Fatal("possibleCapList() missing user provided cap")
	}

	if _, ok = ls["sasl"]; !ok {
		t.Fatal("possibleCapList() missing sasl cap even though auth provided")
	}
}

func TestParseCap(t *testing.T) {
	tests := []struct {
		in   string
		want map[string]map[string]string
	}{
		{in: "sts=port=6697,duration=1234567890,preload", want: map[string]map[string]string{"sts": {"duration": "1234567890", "preload": "", "port": "6697"}}},
		{in: "userhost-in-names", want: map[string]map[string]string{"userhost-in-names": nil}},
		{in: "userhost-in-names test2", want: map[string]map[string]string{"userhost-in-names": nil, "test2": nil}},
		{in: "example/name=test", want: map[string]map[string]string{"example/name": {"test": ""}}},
		{
			in: "userhost-in-names example/name example/name2=test=1,test2=true",
			want: map[string]map[string]string{
				"userhost-in-names": nil,
				"example/name":      nil,
				"example/name2":     {"test": "1", "test2": "true"},
			},
		},
	}

	for _, tt := range tests {
		got := parseCap(tt.in)

		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("parseCap(%q) == %#v :: want %#v", tt.in, got, tt.want)
		}
	}
}

func TestTagGetSetCount(t *testing.T) {
	e := ParseEvent("@aaa=bbb :nick!user@host TAGMSG")
	if e == nil || e.Tags == nil {
		t.Fatal("event for get/set tests didn't parse successfully")
	}

	if e.Tags.Count() != 1 {
		t.Fatalf("Event.Tag.Count() == %d, wanted 1", e.Tags.Count())
	}

	if v, _ := e.Tags.Get("aaa"); v != "bbb" {
		t.Fatalf("Event.Tag.Get('aaa') == %s, wanted bbb", v)
	}

	if err := e.Tags.Set("bbb/test", "test1"); err != nil {
		t.Fatal(err)
	}

	if e.Tags.Count() != 2 {
		t.Fatal("tag count didn't increase after set")
	}

	if v, _ := e.Tags.Get("bbb/test"); v != "test1" {
		t.Fatalf("Event.Tag.Get('bbb/test') == %s, wanted test1", v)
	}

	if ok := e.Tags.Remove("bbb/test"); !ok {
		t.Fatal("tag removal of bbb/test failed")
	}

	if e.Tags.Count() != 1 {
		t.Fatal("tag removal didn't decrease count")
	}

	if err := e.Tags.Set("invalid!key", ""); err == nil {
		t.Fatal("tag set of invalid key should have returned error")
	}

	if err := e.Tags.Set("", ""); err == nil {
		t.Fatal("tag set of empty key should have returned error")
	}

	// Add a hidden ascii value at the end to make it invalid.
	if err := e.Tags.Set("key", "invalid-value\b"); err == nil {
		t.Fatal("tag set of invalid value should have returned error")
	}
}
