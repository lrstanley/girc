// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

var possibleCap = map[string][]string{
	"account-notify":    nil,
	"account-tag":       nil,
	"away-notify":       nil,
	"batch":             nil,
	"cap-notify":        nil,
	"chghost":           nil,
	"extended-join":     nil,
	"message-tags":      nil,
	"multi-prefix":      nil,
	"userhost-in-names": nil,
}

func (c *Client) listCAP() error {
	if !c.config.DisableTracking && !c.config.DisableCapTracking {
		if err := c.write(&Event{Command: CAP, Params: []string{CAP_LS, "302"}}); err != nil {
			return err
		}
	}

	return nil
}

func possibleCapList(c *Client) map[string][]string {
	out := make(map[string][]string)

	for k := range c.config.SupportedCaps {
		out[k] = c.config.SupportedCaps[k]
	}

	for k := range possibleCap {
		out[k] = possibleCap[k]
	}

	return out
}

func parseCap(raw string) map[string][]string {
	out := make(map[string][]string)
	parts := strings.Split(raw, " ")

	var val int

	for i := 0; i < len(parts); i++ {
		val = strings.IndexByte(parts[i], prefixTagValue) // =

		// No value splitter, or has splitter but no trailing value.
		if val < 1 || len(parts[i]) < val+1 {
			// The capability doesn't contain a value.
			out[parts[i]] = []string{}
			continue
		}

		out[parts[i][:val]] = strings.Split(parts[i][val+1:], ",")
	}

	return out
}

// handleCAP attempts to find out what IRCv3 capabilities the server supports.
// This will lock further registration until we have acknowledged the
// capabilities.
func handleCAP(c *Client, e Event) {
	if len(e.Params) >= 2 && (e.Params[1] == CAP_NEW || e.Params[1] == CAP_DEL) {
		c.listCAP()
		return
	}

	// We can assume there was a failure attempting to enable a capability.
	if len(e.Params) == 2 && e.Params[1] == CAP_NAK {
		// Let the server know that we're done.
		c.write(&Event{Command: CAP, Params: []string{CAP_END}})
		return
	}

	possible := possibleCapList(c)

	if len(e.Params) >= 2 && len(e.Trailing) > 1 && e.Params[1] == CAP_LS {
		c.state.mu.Lock()

		caps := parseCap(e.Trailing)

		for k := range caps {
			if _, ok := possible[k]; !ok {
				continue
			}

			if len(possible[k]) == 0 || len(caps[k]) == 0 {
				c.state.tmpCap = append(c.state.tmpCap, k)
				continue
			}

			var contains bool
			for i := 0; i < len(caps[k]); i++ {
				for j := 0; j < len(possible[k]); j++ {
					if caps[k][i] == possible[k][j] {
						// Assume we have a matching split value.
						contains = true
						break
					}

					if contains {
						break
					}
				}

				if contains {
					break
				}
			}

			if !contains {
				continue
			}

			c.state.tmpCap = append(c.state.tmpCap, k)
		}
		c.state.mu.Unlock()

		// Indicates if this is a multi-line LS. (2 args means it's the
		// last LS).
		if len(e.Params) == 2 {
			// If we support no caps, just ack the CAP message and END.
			if len(c.state.tmpCap) == 0 {
				c.write(&Event{Command: CAP, Params: []string{CAP_END}})
				return
			}

			// Let them know which ones we'd like to enable.
			c.write(&Event{Command: CAP, Params: []string{CAP_REQ}, Trailing: strings.Join(c.state.tmpCap, " ")})

			// Re-initialize the tmpCap, so if we get multiple 'CAP LS' requests
			// due to cap-notify, we can re-evaluate what we can support.
			c.state.mu.Lock()
			c.state.tmpCap = []string{}
			c.state.mu.Unlock()
		}
	}

	if len(e.Params) == 2 && len(e.Trailing) > 1 && e.Params[1] == CAP_ACK {
		c.state.mu.Lock()
		c.state.enabledCap = strings.Split(e.Trailing, " ")
		c.state.mu.Unlock()

		// Let the server know that we're done.
		c.write(&Event{Command: CAP, Params: []string{CAP_END}})
		return
	}
}

// handleCHGHOST handles incoming IRCv3 hostname change events. CHGHOST is
// what occurs (when enabled) when a servers services change the hostname of
// a user. Traditionally, this was simply resolved with a quick QUIT and JOIN,
// however CHGHOST resolves this in a much cleaner fashion.
func handleCHGHOST(c *Client, e Event) {
	if len(e.Params) != 2 {
		return
	}

	c.state.mu.Lock()
	users := c.state.lookupUsers("nick", e.Source.Name)

	for i := 0; i < len(users); i++ {
		users[i].Ident = e.Params[0]
		users[i].Host = e.Params[1]
	}
	c.state.mu.Unlock()
}

// handleAWAY handles incoming IRCv3 AWAY events, for which are sent both
// when users are no longer away, or when they are away.
func handleAWAY(c *Client, e Event) {
	c.state.mu.Lock()
	users := c.state.lookupUsers("nick", e.Source.Name)

	for i := 0; i < len(users); i++ {
		users[i].Extras.Away = e.Trailing
	}
	c.state.mu.Unlock()
}

// handleACCOUNT handles incoming IRCv3 ACCOUNT events. ACCOUNT is sent when
// a user logs into an account, logs out of their account, or logs into a
// different account. The account backend is handled server-side, so this
// could be NickServ, X (undernet?), etc.
func handleACCOUNT(c *Client, e Event) {
	if len(e.Params) != 1 {
		return
	}

	account := e.Params[0]
	if account == "*" {
		account = ""
	}

	c.state.mu.Lock()
	users := c.state.lookupUsers("nick", e.Source.Name)

	for i := 0; i < len(users); i++ {
		users[i].Extras.Account = account
	}
	c.state.mu.Unlock()
}

// handleTags handles any messages that have tags that will affect state. (e.g.
// 'account' tags.)
func handleTags(c *Client, e Event) {
	if len(e.Tags) == 0 {
		return
	}

	account, ok := e.Tags.Get("account")
	if !ok {
		return
	}

	c.state.mu.Lock()
	users := c.state.lookupUsers("nick", e.Source.Name)

	for i := 0; i < len(users); i++ {
		users[i].Extras.Account = account
	}
	c.state.mu.Unlock()
}

const (
	prefixTag      byte = 0x40 // @
	prefixTagValue byte = 0x3D // =
	prefixUserTag  byte = 0x2B // +
	tagSeparator   byte = 0x3B // ;
	maxTagLength   int  = 511  // 510 + @ and " " (space), though space usually not included.
)

// Tags represents the key-value pairs in IRCv3 message tags. The map contains
// the encoded message-tag values. If the tag is present, it may still be
// empty. See Tags.Get() and Tags.Set() for use with getting/setting
// information within the tags.
//
// Note that retrieving and setting tags are not concurrent safe. If this is
// necessary, you will need to implement it yourself.
type Tags map[string]string

// ParseTags parses out the key-value map of tags. raw should only be the tag
// data, not a full message. For example:
//   @aaa=bbb;ccc;example.com/ddd=eee
// NOT:
//   @aaa=bbb;ccc;example.com/ddd=eee :nick!ident@host.com PRIVMSG me :Hello
func ParseTags(raw string) (t Tags) {
	t = make(Tags)

	if len(raw) > 0 && raw[0] == prefixTag {
		raw = raw[1:]
	}

	parts := strings.Split(raw, string(tagSeparator))
	var hasValue int

	for i := 0; i < len(parts); i++ {
		hasValue = strings.IndexByte(parts[i], prefixTagValue)

		// The tag doesn't contain a value or has a splitter with no value.
		if hasValue < 1 || len(parts[i]) < hasValue+1 {
			if !validTag(parts[i]) {
				continue
			}

			t[parts[i]] = ""
			continue
		}

		// Check if tag key or decoded value are invalid.
		if !validTag(parts[i][:hasValue]) || !validTagValue(tagDecoder.Replace(parts[i][hasValue+1:])) {
			continue
		}

		t[parts[i][:hasValue]] = parts[i][hasValue+1:]
	}

	return t
}

// Len determines the length of the bytes representation of this tag map. This
// does not include the trailing space required when creating an event, but
// does include the tag prefix ("@").
func (t Tags) Len() (length int) {
	return len(t.Bytes())
}

// Count finds how many total tags that there are.
func (t Tags) Count() int {
	return len(t)
}

// Bytes returns a []byte representation of this tag map, including the tag
// prefix ("@").
func (t Tags) Bytes() []byte {
	max := len(t)
	if max == 0 {
		return nil
	}

	buffer := new(bytes.Buffer)
	buffer.WriteByte(prefixTag)

	var current int

	for tagName, tagValue := range t {
		// Trim at max allowed chars.
		if (buffer.Len() + len(tagName) + len(tagValue) + 2) > maxTagLength {
			return buffer.Bytes()
		}

		buffer.WriteString(tagName)

		// Write the value as necessary.
		if len(tagValue) > 0 {
			buffer.WriteByte(prefixTagValue)
			buffer.WriteString(tagValue)
		}

		// add the separator ";" between tags.
		if current <= max {
			buffer.WriteByte(tagSeparator)
		}

		current++
	}

	return buffer.Bytes()
}

// String returns a string representation of this tag map.
func (t Tags) String() string {
	return string(t.Bytes())
}

// writeTo writes the necessary tag bytes to an io.Writer, including a trailing
// space-separator.
func (t Tags) writeTo(w io.Writer) (n int, err error) {
	b := t.Bytes()
	if len(b) == 0 {
		return n, err
	}

	n, err = w.Write(b)
	if err != nil {
		return n, err
	}

	var j int
	j, err = w.Write([]byte{eventSpace})
	n += j

	return n, err
}

// tagDecode are encoded -> decoded pairs for replacement to decode.
var tagDecode = []string{
	"\\:", ";",
	"\\s", " ",
	"\\\\", "\\",
	"\\r", "\r",
	"\\n", "\n",
}
var tagDecoder = strings.NewReplacer(tagDecode...)

// tagEncode are decoded -> encoded pairs for replacement to decode.
var tagEncode = []string{
	";", "\\:",
	" ", "\\s",
	"\\", "\\\\",
	"\r", "\\r",
	"\n", "\\n",
}
var tagEncoder = strings.NewReplacer(tagEncode...)

// Get returns the unescaped value of given tag key. Note that this is not
// concurrent safe.
func (t Tags) Get(key string) (tag string, success bool) {
	if _, ok := t[key]; ok {
		tag = tagDecoder.Replace(t[key])
		success = true
	}

	return tag, success
}

// Set escapes given value and saves it as the value for given key. Note that
// this is not concurrent safe.
func (t Tags) Set(key, value string) error {
	if !validTag(key) {
		return fmt.Errorf("tag %q is invalid", key)
	}

	value = tagEncoder.Replace(value)

	// Check to make sure it's not too long here.
	if (t.Len() + len(key) + len(value) + 2) > maxTagLength {
		return fmt.Errorf("unable to set tag %q [value %q]: tags too long for message", key, value)
	}

	t[key] = value

	return nil
}

// Remove deletes the tag frwom the tag map.
func (t Tags) Remove(key string) (success bool) {
	if _, success = t[key]; success {
		delete(t, key)
	}

	return success
}

// validTag validates an IRC tag.
func validTag(name string) bool {
	if len(name) < 1 {
		return false
	}

	// Allow user tags to be passed to validTag.
	if len(name) >= 2 && name[0] == prefixUserTag {
		name = name[1:]
	}

	for i := 0; i < len(name); i++ {
		// A-Z, a-z, 0-9, -/._
		if (name[i] < 0x41 || name[i] > 0x5A) && (name[i] < 0x61 || name[i] > 0x7A) && (name[i] < 0x2D || name[i] > 0x39) && name[i] != 0x5F {
			return false
		}
	}

	return true
}

// validTagValue valids a decoded IRC tag value. If the value is not decoded
// with tagDecoder first, it may be seen as invalid.
func validTagValue(value string) bool {
	for i := 0; i < len(value); i++ {
		// Don't allow any invisible chars within the tag, or semicolons.
		if value[i] < 0x21 || value[i] > 0x7E || value[i] == 0x3B {
			return false
		}
	}
	return true
}
