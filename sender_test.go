// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bytes"
	"testing"
	"time"
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

func TestEventLimiter(t *testing.T) {
	events := []*Event{}
	sendFunc := func(event *Event) error {
		events = append(events, event)

		return nil
	}

	limiter := NewEventLimiter(1, 150*time.Millisecond, sendFunc)

	var e1, e2 *Event
	go func() {
		if err := limiter.SendAll(e1, e2); err != nil {
			t.Fatalf("SendAll gave: %v", err)
		}
	}()

	// Checking it immediately should yield 1 time.
	if len(events) > 1 {
		t.Fatalf("limiter has %v events, wanted 0 or 1", len(events))
	}

	time.Sleep(500 * time.Millisecond)

	// It should now show a length of two.
	if len(events) != 2 {
		t.Fatalf("limiter has %v events, wanted 2", len(events))
	}

	limiter.Stop()
}
