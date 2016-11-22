// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"reflect"
	"testing"
)

func TestParseSource(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name    string
		args    args
		wantSrc *Source
	}{
		{name: "full", args: args{raw: "nick!user@hostname.com"}, wantSrc: &Source{
			Name: "nick", User: "user", Host: "hostname.com",
		}},
		{name: "special chars", args: args{raw: "^[]nick!~user@test.host---name.com"}, wantSrc: &Source{
			Name: "^[]nick", User: "~user", Host: "test.host---name.com",
		}},
		{name: "short", args: args{raw: "a!b@c"}, wantSrc: &Source{
			Name: "a", User: "b", Host: "c",
		}},
		{name: "short", args: args{raw: "a!b"}, wantSrc: &Source{
			Name: "a", User: "b", Host: "",
		}},
		{name: "short", args: args{raw: "a@b"}, wantSrc: &Source{
			Name: "a", User: "", Host: "b",
		}},
		{name: "short", args: args{raw: "test"}, wantSrc: &Source{
			Name: "test", User: "", Host: "",
		}},
	}
	for _, tt := range tests {
		gotSrc := ParseSource(tt.args.raw)

		if !reflect.DeepEqual(gotSrc, tt.wantSrc) {
			t.Errorf("ParseSource() = %v, want %v", gotSrc, tt.wantSrc)
		}

		if gotSrc.Len() != tt.wantSrc.Len() {
			t.Errorf("ParseSource().Len() = %v, want %v", gotSrc.Len(), tt.wantSrc.Len())
		}

		if gotSrc.String() != tt.wantSrc.String() {
			t.Errorf("ParseSource().String() = %v, want %v", gotSrc.String(), tt.wantSrc.String())
		}

		if gotSrc.IsServer() != tt.wantSrc.IsServer() {
			t.Errorf("ParseSource().IsServer() = %v, want %v", gotSrc.IsServer(), tt.wantSrc.IsServer())
		}

		if gotSrc.IsHostmask() != tt.wantSrc.IsHostmask() {
			t.Errorf("ParseSource().IsHostmask() = %v, want %v", gotSrc.IsHostmask(), tt.wantSrc.IsHostmask())
		}

		if !reflect.DeepEqual(gotSrc.Bytes(), tt.wantSrc.Bytes()) {
			t.Errorf("ParseSource().Bytes() = %v, want %v", gotSrc, tt.wantSrc)
		}
	}
}
