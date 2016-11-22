// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bytes"
	"testing"
)

func TestSender(t *testing.T) {
	bw := &bytes.Buffer{}
	writer := newEncoder(bw)
	s := serverSender{writer: writer}

	e := &Event{Command: "TEST"}
	s.Send(e)

	if e.Raw()+"\r\n" != bw.String() {
		t.Errorf("serverSender{writer: newEncoder(bytes.Buffer)} = %v, want %v", bw, e.String()+"\r\n")
	}
}
