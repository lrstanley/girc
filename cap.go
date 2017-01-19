// Copyright 2016-2017 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

var possibleCap = []string{
	"account-notify",
	"account-tag",
	"away-notify",
	"batch",
	"cap-notify",
	"chghost",
	"message-tags",
}

func (c *Client) listCAP() error {
	if !c.Config.DisableTracking && !c.Config.DisableCapTracking {
		if err := c.write(&Event{Command: CAP, Params: []string{CAP_LS, "302"}}); err != nil {
			return err
		}
	}

	return nil
}

// handleCAP attempts to find out what IRCv3 capabilities the server supports.
// This will lock further registration until we have acknowledged the
// capabilities.
func handleCAP(c *Client, e Event) {
	possible := c.Config.SupportedCaps
	possible = append(possible, possibleCap...)

	if len(e.Params) >= 2 && (e.Params[1] == CAP_NEW || e.Params[1] == CAP_DEL) {
		c.listCAP()
		return
	}

	// We can assume there was a failure attempting to enable a capability.
	if len(e.Params) == 2 && e.Params[1] == CAP_NAK {
		// Let the server know that we're done.
		c.Send(&Event{Command: CAP, Params: []string{CAP_END}})
		return
	}

	if len(e.Params) >= 2 && len(e.Trailing) > 1 && e.Params[1] == CAP_LS {
		c.state.mu.Lock()
		// Loop through and check if it's one we support.
		for _, cap := range strings.Split(e.Trailing, " ") {
			for i := 0; i < len(possible); i++ {
				// Check if it's one that we support.
				if possibleCap[i] == cap {
					// Ensure that there are no duplicates.
					var isin bool
					for j := 0; j < len(c.state.tmpCap); j++ {
						if c.state.tmpCap[j] == cap {
							isin = true
							break
						}
					}
					if !isin {
						c.state.tmpCap = append(c.state.tmpCap, cap)
					}

					break
				}
			}
		}
		c.state.mu.Unlock()

		// Indicates if this is a multi-line LS. (2 args means it's the
		// last LS).
		if len(e.Params) == 2 {
			// If we support no caps, just ack the CAP message and END.
			if len(c.state.tmpCap) == 0 {
				c.Send(&Event{Command: CAP, Params: []string{CAP_END}})
				return
			}

			// Let them know which ones we'd like to enable.
			c.Send(&Event{Command: CAP, Params: []string{CAP_REQ}, Trailing: strings.Join(c.state.tmpCap, " ")})

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
		c.Send(&Event{Command: CAP, Params: []string{CAP_END}})
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
	users := c.state.getUsers("nick", e.Source.Name)

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
	users := c.state.getUsers("nick", e.Source.Name)

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
	users := c.state.getUsers("nick", e.Source.Name)

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
	users := c.state.getUsers("nick", e.Source.Name)

	for i := 0; i < len(users); i++ {
		users[i].Extras.Account = account
	}
	c.state.mu.Unlock()
}

const (
	prefixTag      byte = 0x40 // @
	prefixTagValue byte = 0x3D // =
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
	parts := strings.Split(raw, string(tagSeparator))
	var hasValue int

	for i := 0; i < len(parts); i++ {
		hasValue = strings.IndexByte(parts[i], prefixTagValue)

		if hasValue < 1 {
			// The tag doesn't contain a value.
			t[parts[i]] = ""
			continue
		}

		// May have equals sign and no value as well.
		if len(parts[i]) < hasValue+1 {
			t[parts[i]] = ""
			continue
		}

		t[parts[i][:hasValue]] = parts[i][hasValue+1:]
		continue
	}

	return t
}

// Len determines the length of the string representation of this tag map.
func (t Tags) Len() (length int) {
	return len(t.String())
}

// Count finds how many total tags that there are.
func (t Tags) Count() int {
	return len(t)
}

// Bytes returns a []byte representation of this tag map.
func (t Tags) Bytes() []byte {
	max := len(t)
	if max == 0 {
		return nil
	}

	buffer := new(bytes.Buffer)
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

	for i := 0; i < len(name); i++ {
		// A-Z, a-z, 0-9, -/._
		if (name[i] < 0x41 || name[i] > 0x5A) && (name[i] < 0x61 || name[i] > 0x7A) && (name[i] < 0x2D || name[i] > 0x39) && name[i] != 0x5F {
			return false
		}
	}

	return true
}
