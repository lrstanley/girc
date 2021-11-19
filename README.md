<p align="center"><a href="https://tcp.ac/i/G5OTn" target="_blank"><img width="270" src="https://tcp.ac/i/G5OTn"></a></p>
<p align="center">girc-atomic, a terrifying fork of an IRC library for Go</p>
<p align="center">
  <a href="https://godoc.org/github.com/yunginnanet/girc-atomic"><img src="https://godoc.org/github.com/yunginnanet/girc-atomic?status.png" alt="GoDoc"></a>
  <a href="https://goreportcard.com/report/github.com/yunginnanet/girc-atomic"><img src="https://goreportcard.com/badge/github.com/yunginnanet/girc-atomic" alt="Go Report Card"></a>
  <a href="ircs://ircd.chat:6697/#tcpdirect"><img src="https://img.shields.io/badge/ircd.chat-%23tcpdirect-blue.svg" alt="IRC Chat"></a>
  <a href=""https://github.com/yunginnanet/girc-atomic/actions/workflows/go.yml"><img src="https://github.com/yunginnanet/girc-atomic/actions/workflows/go.yml/badge.svg?branch=master" alt="Build Status"></a>
</p>

## Fork changes

[Click here to see the changes in girc-atomic vs girc](https://github.com/lrstanley/girc/compare/master...yunginnanet:master)

## Status
  
### ₜₕₑ ₛₖy ᵢₛ 𝆑ₐₗₗᵢₙg ʇɥǝ sʞʎ ᴉs ⅎɐʅʅᴉuƃ 
### 𝚝𝚑𝚎𝚢 𝚜𝚑𝚘𝚞𝚕𝚍 𝚑𝚊𝚟𝚎 𝚕𝚒𝚜𝚝𝚎𝚗𝚎𝚍
### ʇɥǝ sʞʎ ᴉs ⅎɐʅʅᴉuƃ ₜₕₑ ₛₖy ᵢₛ 𝆑ₐₗₗᵢₙg  

  
~~girc is fairly close to marking the 1.0.0 endpoint, which will be tagged as
necessary, so you will be able to use this with care knowing the specific tag
you're using won't have breaking changes~~

## Features

- Focuses on ~~simplicity~~ ʀᴀɪɴɪɴɢ ʜᴇʟʟғɪʀᴇ, yet tries to still be flexible.
- Only requires [standard library packages](https://godoc.org/github.com/yunginnanet/girc-atomic?imports)
- Event based triggering/responses ([example](https://godoc.org/github.com/yunginnanet/girc-atomic#ex-package--Commands), and [CTCP too](https://godoc.org/github.com/yunginnanet/girc-atomic#Commands.SendCTCP)!)
- [Documentation](https://godoc.org/github.com/yunginnanet/girc-atomic) is _mostly_ complete.
- Support for almost all of the [IRCv3 spec](http://ircv3.net/software/libraries.html).
  - SASL Auth (currently only `PLAIN` and `EXTERNAL` is support by default,
  however you can simply implement `SASLMech` yourself to support additional
  mechanisms.)
  - Message tags (things like `account-tag` on by default)
  - `account-notify`, `away-notify`, `chghost`, `extended-join`, etc -- all handled seemlessly ([cap.go](https://github.com/yunginnanet/girc-atomic/blob/master/cap.go) for more info).
- Channel and user tracking. Easily find what users are in a channel, if a
  user is away, or if they are authenticated (if the server supports it!)
- Client state/capability tracking. Easy methods to access capability data ([LookupChannel](https://godoc.org/github.com/yunginnanet/girc-atomic#Client.LookupChannel), [LookupUser](https://godoc.org/github.com/yunginnanet/girc-atomic#Client.LookupUser), [GetServerOption (ISUPPORT)](https://godoc.org/github.com/yunginnanet/girc-atomic#Client.GetServerOption), etc.)
- Built-in support for things you would commonly have to implement yourself.
  - Nick collision detection and prevention (also see [Config.HandleNickCollide](https://godoc.org/github.com/yunginnanet/girc-atomic#Config).)
  - Event/message rate limiting.
  - Channel, nick, and user validation methods ([IsValidChannel](https://godoc.org/github.com/yunginnanet/girc-atomic#IsValidChannel), [IsValidNick](https://godoc.org/github.com/yunginnanet/girc-atomic#IsValidNick), etc.)
  - CTCP handling and auto-responses ([CTCP](https://godoc.org/github.com/yunginnanet/girc-atomic#CTCP))
  - ~~And more!~~  
  - GOTTA GO FAST YOU GOTTA GO REALLY FAST
  - you can power hundreds of clients at the same time with this now

## Installing

  ~~$ go get -u github.com/yunginnanet/girc-atomic~~  
  just use go modules probably

## Examples

See [the examples](https://godoc.org/github.com/yunginnanet/girc-atomic#example-package--Bare)
within the documentation for real-world usecases. Here are a few real-world
usecases/examples/projects which utilize the real girc:

| Project | Description |
| --- | --- |
| [nagios-check-ircd](https://github.com/lrstanley/nagios-check-ircd) | Nagios utility for monitoring the health of an ircd |
| [nagios-notify-irc](https://github.com/lrstanley/nagios-notify-irc) | Nagios utility for sending alerts to one or many channels/networks |
| [matterbridge](https://github.com/42wim/matterbridge) | bridge between mattermost, IRC, slack, discord (and many others) with REST API |

Working on a project and want to add it to the list? Submit a pull request!

## Contributing

~~Please review the [CONTRIBUTING](CONTRIBUTING.md) doc for submitting issues/a guide
on submitting pull requests and helping out.~~  
  
**OH GOD PLEASE MAKE IT STOP**


## License

    Copyright (c) 2016 Liam Stanley <me@liamstanley.io>

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in all
    copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
    SOFTWARE.

girc artwork licensed under [CC 3.0](http://creativecommons.org/licenses/by/3.0/) based on Renee French under Creative Commons 3.0 Attributions...  
  
   or so I'm told. Then it was defiled by [some idiot](https://github.com/yunginnanet).

## References

   * [IRCv3: Specification Docs](http://ircv3.net/irc/)
   * [IRCv3: Specification Repo](https://github.com/ircv3/ircv3-specifications)
   * [IRCv3 Capability Registry](http://ircv3.net/registry.html)
   * [IRCv3: WEBIRC](https://ircv3.net/specs/extensions/webirc.html)
   * [KiwiIRC: WEBIRC](https://kiwiirc.com/docs/webirc)
   * [ISUPPORT Specification Docs](http://www.irc.org/tech_docs/005.html) ([alternative 1](http://defs.ircdocs.horse/defs/isupport.html), [alternative 2](https://github.com/grawity/irc-docs/blob/master/client/RPL_ISUPPORT/draft-hardy-irc-isupport-00.txt), [relevant draft](http://www.irc.org/tech_docs/draft-brocklesby-irc-isupport-03.txt))
   * [IRC Numerics List](http://defs.ircdocs.horse/defs/numerics.html)
   * [Extended WHO (also known as WHOX)](https://github.com/quakenet/snircd/blob/master/doc/readme.who)
   * [RFC1459: Internet Relay Chat Protocol](https://tools.ietf.org/html/rfc1459)
   * [RFC2812: Internet Relay Chat: Client Protocol](https://tools.ietf.org/html/rfc2812)
   * [RFC2813: Internet Relay Chat: Server Protocol](https://tools.ietf.org/html/rfc2813)
   * [RFC7194: Default Port for Internet Relay Chat (IRC) via TLS/SSL](https://tools.ietf.org/html/rfc7194)
   * [RFC4422: Simple Authentication and Security Layer](https://tools.ietf.org/html/rfc4422) ([SASL EXTERNAL](https://tools.ietf.org/html/rfc4422#appendix-A))
   * [RFC4616: The PLAIN SASL Mechanism](https://tools.ietf.org/html/rfc4616)
