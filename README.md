## girc is a flexible IRC library for Go

[![Build Status](https://travis-ci.org/lrstanley/girc.svg?branch=master)](https://travis-ci.org/lrstanley/girc)
[![GoDoc](https://godoc.org/github.com/lrstanley/girc?status.png)](https://godoc.org/github.com/lrstanley/girc)
[![codebeat badge](https://codebeat.co/badges/9f69c452-abc4-4dd3-9e9c-8536ad0a7b18)](https://codebeat.co/projects/github-com-lrstanley-girc)
[![Go Report Card](https://goreportcard.com/badge/github.com/lrstanley/girc)](https://goreportcard.com/report/github.com/lrstanley/girc)
[![IRC Chat](https://img.shields.io/badge/ByteIRC-%23L-blue.svg)](http://byteirc.org/channel/L)
[![GitHub Issues](https://img.shields.io/github/issues/lrstanley/girc.svg)](https://github.com/lrstanley/girc/issues)
[![license](https://img.shields.io/github/license/lrstanley/girc.svg)](https://raw.githubusercontent.com/lrstanley/girc/master/LICENSE)

## Features

- Focuses on simplicity, yet tries to still be flexible
- Only requires standard packages
- Event based triggering/responses
- Documentation is mostly on par
- At this time, **expect breaking changes to occur frequently**.

## TODO

- [ ] less potnetial for data races in `state.go`
- [ ] add JoinKey, re-setup Join args
- [ ] implement `Who()` which actually returns results
- [ ] make sure client can easily be garbage collected
- [ ] ensure types `User` and `Channel` don't have any unexported fields, and that when they are given publically, it's not a pointer to internal state
- [ ] track with `NAMES` as well? would require rewrite of user existance logic, could also help track user modes
- [ ] write more function-specific examples as the api becomes much more stable
- [ ] would be cool to track things like `SERVERNAME`, `VERSION`, `UMODES`, `CMODES`, etc. also see `Config.DisableCapTracking`. [e.g. here](https://github.com/lrstanley/Code/blob/master/core/triggers.py#L40-L67)
- [ ] client should support ping tracking (sending `PING`'s to the server)
  - [ ] with this, we can potentially find lag. `Client.Lag()` would be useful
- [ ] users need to be exposed in state somehow (other than `GetChannels()`)
- [ ] `User.Age()`? (`FirstActive()`?) (time since first seen)
- [ ] `MODE` tracking on a per-channel basis
- [ ] `Client.AddTmpCallback()` for one time use callbacks?

## Installing

    $ go get -u github.com/lrstanley/girc

## Examples

See [the examples](https://godoc.org/github.com/lrstanley/girc#example-package) within the documentation for real-world usecases.

## Contributing

Below are a few guidelines if you would like to contribute. Keep the code clean, standardized, and much of the quality should match Golang's standard library and common idioms.

   * Always test using the latest Go version.
   * Always use `gofmt` before committing anything.
   * Always have proper documentation before committing.
   * Keep the same whitespacing, documentation, and newline format as the rest of the project.
   * Only use 3rd party libraries if necessary. If only a small portion of the library is needed, simply rewrite it within the library to prevent useless imports.
   * Also see [golang/go/wiki/CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)

## License

```
The MIT License (MIT); Copyright (c) Liam Stanley <me@liamstanley.io>
```
