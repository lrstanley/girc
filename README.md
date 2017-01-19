<p align="center"><a href="https://godoc.org/github.com/lrstanley/girc"><img  width="600" src="https://i.imgur.com/Wh6otgh.png"></a></p>
<p align="center">girc -- A flexible IRC library for Go</p>
<p align="center">
  <a href="https://travis-ci.org/lrstanley/girc"><img src="https://travis-ci.org/lrstanley/girc.svg?branch=master" alt="Build Status"></a>
  <a href="https://godoc.org/github.com/lrstanley/girc"><img src="https://godoc.org/github.com/lrstanley/girc?status.png" alt="GoDoc"></a>
  <a href="https://goreportcard.com/report/github.com/lrstanley/girc"><img src="https://goreportcard.com/badge/github.com/lrstanley/girc" alt="Go Report Card"></a>
  <a href="http://byteirc.org/channel/L"><img src="https://img.shields.io/badge/ByteIRC-%23L-blue.svg" alt="IRC Chat"></a>
  <a href="https://github.com/lrstanley/girc/issues"><img src="https://img.shields.io/github/issues/lrstanley/girc.svg" alt="GitHub Issues"></a>
  <a href="https://raw.githubusercontent.com/lrstanley/girc/master/LICENSE"><img src="https://img.shields.io/github/license/lrstanley/girc.svg" alt="License"></a>
</p>

## Features

- Focuses on simplicity, yet tries to still be flexible.
- Only requires standard library packages.
- Event based triggering/responses (and CTCP too!).
- Documentation is mostly on par.
- Full support for the IRCv3 spec. [**WIP**]
- Channel and user tracking. Easily find what users are in a channel, if a
  user is away, or if they are authenticated.
- Client state/capability tracking. Easy methods to access capability data.
- Built-in support for things you would commmonly have to implement yourself.
  - Nick collision detection and prevention.
  - Event/message rate limiting.
  - Channel, nick, and user validation on connection methods.
  - CTCP handling and auto-responses.

- At this time, **expect breaking changes to occur frequently**. girc has **not hit version 1.0.0 yet!**

## TODO

- [ ] IRCv3 spec -- [details](http://ircv3.net):
  - [ ] [multi-prefix](http://ircv3.net/specs/extensions/multi-prefix-3.1.html)
  - [ ] [sasl](http://ircv3.net/specs/extensions/sasl-3.2.html)
  - [ ] [userhost-in-names](http://ircv3.net/specs/extensions/userhost-in-names-3.2.html)
  - [ ] [extended-join](http://ircv3.net/specs/extensions/extended-join-3.1.html)
- [ ] ensure types `User` and `Channel` don't have any unexported fields, and
      that when they are given publically, it's not a pointer to internal
      state.
- [ ] track with `NAMES` as well? would require rewrite of user existance
      logic, could also help track user modes.
- [ ] write more function-specific examples as the api becomes much more stable
- [ ] client should support ping tracking (sending `PING`'s to the server)
  - [ ] with this, we can potentially find lag. `Client.Lag()` would be useful
  - [ ] allow support for changing the frequency of this?
- [ ] users need to be exposed in state some how (other than `GetChannels()`)
- [ ] `MODE` tracking on a per-channel basis
- [ ] `Client.AddTmpCallback()` for one time use callbacks?
- [ ] allow support for proxy URLs (passing to `golang.org/x/net/proxy`?)
- [ ] allow users to specify a local/bind address using `net.Dialer{}.LocalAddr`
- [ ] add more generic helpers: `Away()`, `Invite()`, `Kick()`, `Oper()`,
      generic `Ping()` and `Pong()`, `VHost()`, `Whois()` and `Who()`

## Installing

    $ go get -u github.com/lrstanley/girc

## Examples

See [the examples](https://godoc.org/github.com/lrstanley/girc#example-package)
within the documentation for real-world usecases.

## Contributing

Below are a few guidelines if you would like to contribute. Keep the code
clean, standardized, and much of the quality should match Golang's standard
library and common idioms.

   * Always test using the latest Go version.
   * Always use `gofmt` before committing anything.
   * Always have proper documentation before committing.
   * Keep the same whitespacing, documentation, and newline format as the
     rest of the project.
   * Only use 3rd party libraries if necessary. If only a small portion of
     the library is needed, simply rewrite it within the library to prevent
     useless imports.
   * Also see [golang/go/wiki/CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)

## License

```
The MIT License (MIT); Copyright (c) Liam Stanley <me@liamstanley.io>
```
