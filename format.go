// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import "strings"

type color struct {
	aliases []string
	val     string
}

var colors = []*color{
	{aliases: []string{"white"}, val: "\x0300"},
	{aliases: []string{"black"}, val: "\x0301"},
	{aliases: []string{"blue", "navy"}, val: "\x0302"},
	{aliases: []string{"green"}, val: "\x0303"},
	{aliases: []string{"red"}, val: "\x0304"},
	{aliases: []string{"brown", "maroon"}, val: "\x0305"},
	{aliases: []string{"purple"}, val: "\x0306"},
	{aliases: []string{"orange", "olive", "gold"}, val: "\x0307"},
	{aliases: []string{"yellow"}, val: "\x0308"},
	{aliases: []string{"lightgreen", "lime"}, val: "\x0309"},
	{aliases: []string{"teal"}, val: "\x0310"},
	{aliases: []string{"cyan"}, val: "\x0311"},
	{aliases: []string{"lightblue", "royal"}, val: "\x0312"},
	{aliases: []string{"lightpurple", "pink", "fuchsia"}, val: "\x0313"},
	{aliases: []string{"grey", "gray"}, val: "\x0314"},
	{aliases: []string{"lightgrey", "silver"}, val: "\x0315"},
	{aliases: []string{"bold", "b"}, val: "\x02"},
	{aliases: []string{"italic", "i"}, val: "\x1d"},
	{aliases: []string{"reset", "r"}, val: "\x0f"},
	{aliases: []string{"clear", "c"}, val: "\x03"},
	{aliases: []string{"reverse"}, val: "\x16"},
	{aliases: []string{"underline", "ul"}, val: "\x1f"},
}

// Format takes color strings like "{red}" and turns them into the resulting
// ASCII color code for IRC.
func Format(text string) string {
	for i := 0; i < len(colors); i++ {
		for a := 0; a < len(colors[i].aliases); a++ {
			text = strings.Replace(text, "{"+colors[i].aliases[a]+"}", colors[i].val, -1)
		}

		// makes parsing small strings slightly slower, but helps longer
		// strings.
		var more bool
		for c := 0; c < len(text); c++ {
			if text[c] == 0x7B {
				more = true
				break
			}
		}
		if !more {
			return text
		}
	}

	return text
}

// StripFormat strips all "{color}" formatting strings from the input text.
// See Format() for more information.
func StripFormat(text string) string {
	for i := 0; i < len(colors); i++ {
		for a := 0; a < len(colors[i].aliases); a++ {
			text = strings.Replace(text, "{"+colors[i].aliases[a]+"}", "", -1)
		}

		// makes parsing small strings slightly slower, but helps longer
		// strings.
		var more bool
		for c := 0; c < len(text); c++ {
			if text[c] == 0x7B {
				more = true
				break
			}
		}
		if !more {
			return text
		}
	}

	return text
}

// StripColors tries to strip all ASCII color codes that are used for IRC.
func StripColors(text string) string {
	for i := 0; i < len(colors); i++ {
		text = strings.Replace(text, colors[i].val, "", -1)
	}

	return text
}
