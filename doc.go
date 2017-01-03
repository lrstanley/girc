// Copyright 2016-2017 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

// Package girc provides a high level, yet flexible IRC library for use with
// interacting with IRC servers. girc has support for user/channel tracking,
// as well as a few other neat features (like auto-reconnect).
//
// Much of what girc can do, can also be disabled. The goal is to provide a
// solid API that you don't necessarily have to work with out of the box if
// you don't want to.
//
// See "examples/simple/main.go" for a brief and very useful example taking
// advantage of girc, that should give you a general idea of how the API
// works.
package girc
