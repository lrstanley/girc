<p align="center"><a href="https://godoc.org/github.com/lrstanley/girc"><img  width="600" src="https://i.imgur.com/Wh6otgh.png"></a></p>
<p align="center">girc -- A flexible IRC library for Go</p>
<p align="center">
  <a href="https://travis-ci.org/lrstanley/girc"><img src="https://travis-ci.org/lrstanley/girc.svg?branch=master" alt="Build Status"></a>
  <a href="https://godoc.org/github.com/lrstanley/girc"><img src="https://godoc.org/github.com/lrstanley/girc?status.png" alt="GoDoc"></a>
  <a href="https://goreportcard.com/report/github.com/lrstanley/girc"><img src="https://goreportcard.com/badge/github.com/lrstanley/girc" alt="Go Report Card"></a>
  <a href="http://byteirc.org/channel/L"><img src="https://img.shields.io/badge/ByteIRC-%23L-blue.svg" alt="IRC Chat"></a>
</p>

## Status

**EXPECT BREAKING CHANGES TO OCCUR FREQUENTLY**. girc has **not hit version
1.0.0 yet!**

Changes are actively being made. At this time, most of the stateful parts of
girc are not accessible, as well as other minor consistencies while things
are still being flushed out. Not production ready! **_You've been warned!_**

## Features

- Focuses on simplicity, yet tries to still be flexible.
- Only requires standard library packages.
- Event based triggering/responses (and CTCP too!).
- Documentation is mostly on par.
- Full support for the IRCv3 spec.
- Channel and user tracking. Easily find what users are in a channel, if a
  user is away, or if they are authenticated.
- Client state/capability tracking. Easy methods to access capability data.
- Built-in support for things you would commmonly have to implement yourself.
  - Nick collision detection and prevention.
  - Event/message rate limiting.
  - Channel, nick, and user validation on connection methods.
  - CTCP handling and auto-responses.

## TODO

To review what is currently being worked on, or looked into, feel free to head
over to the [project board](https://github.com/lrstanley/girc/projects/1) or
the [issues list](https://github.com/lrstanley/girc/issues).

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
LICENSE: The MIT License (MIT)
Copyright (c) Liam Stanley <me@liamstanley.io>

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
