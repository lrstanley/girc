<p align="center"><a href="https://godoc.org/github.com/lrstanley/girc"><img  width="600" src="https://i.imgur.com/Wh6otgh.png"></a></p>
<p align="center">girc -- A flexible IRC library for Go</p>
<p align="center">
  <a href="https://travis-ci.org/lrstanley/girc"><img src="https://travis-ci.org/lrstanley/girc.svg?branch=master" alt="Build Status"></a>
  <a href="http://gocover.io/github.com/lrstanley/girc"><img src="https://coveralls.io/repos/github/lrstanley/girc/badge.svg?branch=master" alt="Coverage Status"></a>
  <a href="https://godoc.org/github.com/lrstanley/girc"><img src="https://godoc.org/github.com/lrstanley/girc?status.png" alt="GoDoc"></a>
  <a href="https://goreportcard.com/report/github.com/lrstanley/girc"><img src="https://goreportcard.com/badge/github.com/lrstanley/girc" alt="Go Report Card"></a>
  <a href="https://byteirc.org/channel/L"><img src="https://img.shields.io/badge/ByteIRC-%23L-blue.svg" alt="IRC Chat"></a>
</p>

## Status

**girc is fairly close to marking the 1.0.0 endpoint, which will be tagged as
necessary, so you will be able to use this with care knowing the specific tag
you're using won't have breaking changes**

## Features

- Focuses on simplicity, yet tries to still be flexible.
- Only requires [standard library packages](https://godoc.org/github.com/lrstanley/girc?imports)
- Event based triggering/responses ([example](https://godoc.org/github.com/lrstanley/girc#ex-package--Commands), and [CTCP too](https://godoc.org/github.com/lrstanley/girc#Commands.SendCTCP)!).
- [Documentation](https://godoc.org/github.com/lrstanley/girc) is _mostly_ complete.
- Support for almost all of the IRCv3 spec.
  - SASL Auth (currently only `PLAIN` and `EXTERNAL` is support by default,
  however you can simply implement `SASLMech` yourself to support additional
  mechanisms.)
  - Message tags (and with this, things like `account-tag` on by default)
  - `account-notify`, `away-notify`, `chghost`, `extended-join`, etc -- all handled seemlessly ([cap.go](https://github.com/lrstanley/girc/blob/master/cap.go) for more info).
- Channel and user tracking. Easily find what users are in a channel, if a
  user is away, or if they are authenticated (if the server supports it!)
- Client state/capability tracking. Easy methods to access capability data ([Lookup](https://godoc.org/github.com/lrstanley/girc#Client.Lookup), [Event.GetChannel](https://godoc.org/github.com/lrstanley/girc#Event.GetChannel), [GetServerOption (ISUPPORT)](https://godoc.org/github.com/lrstanley/girc#Client.GetServerOption), etc.)
- Built-in support for things you would commonly have to implement yourself.
  - Nick collision detection and prevention (also see [Config.HandleNickCollide](https://godoc.org/github.com/lrstanley/girc#Config).)
  - Event/message rate limiting.
  - Channel, nick, and user validation on connection methods ([IsValidChannel](https://godoc.org/github.com/lrstanley/girc#IsValidChannel), [IsValidNick](https://godoc.org/github.com/lrstanley/girc#IsValidNick), etc.)
  - CTCP handling and auto-responses ([CTCP](https://godoc.org/github.com/lrstanley/girc#CTCP).)
  - And more!

## Installing

    $ go get -u github.com/lrstanley/girc

## Examples

See [the examples](https://godoc.org/github.com/lrstanley/girc#example-package--Bare)
within the documentation for real-world usecases. Here are a few real-world
usecases/examples/projects which utilize girc:

| Project | Description |
| --- | --- |
| [nagios-check-ircd](github.com/lrstanley/nagios-check-ircd) | Nagios script for monitoring the health of an ircd |

Working on a project and want to add it to the list? Submit a pull request!

## Contributing

Please review the [CONTRIBUTING](https://github.com/lrstanley/girc/blob/master/README.md)
doc for submitting issues/a guide on submitting pull requests and helping out.

## License

```
LICENSE: The MIT License (MIT)
Copyright (c) 2016 Liam Stanley <me@liamstanley.io>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## References

   * [rfc1459: Internet Relay Chat Protocol](https://tools.ietf.org/html/rfc1459)
   * [rfc2812: Internet Relay Chat: Client Protocol](https://tools.ietf.org/html/rfc2812)
   * [rfc2813: Internet Relay Chat: Server Protocol](https://tools.ietf.org/html/rfc2813)
   * [rfc7194: Default Port for Internet Relay Chat (IRC) via TLS/SSL](https://tools.ietf.org/html/rfc7194)
   * [IRCv3: Specification Docs](http://ircv3.net/irc/)
   * [IRCv3: Specification Repo](https://github.com/ircv3/ircv3-specifications)
   * [IRCv3 Capability Registry](http://ircv3.net/registry.html)
   * [Extended WHO (also known as WHOX)](https://github.com/quakenet/snircd/blob/master/doc/readme.who)
