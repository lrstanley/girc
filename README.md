## girc is a flexible IRC library for Go

[![Build Status](https://travis-ci.org/Liamraystanley/girc.svg?branch=master)](https://travis-ci.org/Liamraystanley/girc)
[![GoDoc](https://godoc.org/github.com/Liamraystanley/girc?status.png)](https://godoc.org/github.com/Liamraystanley/girc)
[![codebeat badge](https://codebeat.co/badges/67d01d61-d5e9-4854-ae22-0ac262dd7690)](https://codebeat.co/projects/github-com-liamraystanley-girc)
[![Go Report Card](https://goreportcard.com/badge/github.com/Liamraystanley/girc)](https://goreportcard.com/report/github.com/Liamraystanley/girc)
[![IRC Chat](https://img.shields.io/badge/ByteIRC-%23L-blue.svg)](http://byteirc.org/channel/L)
[![GitHub Issues](https://img.shields.io/github/issues/Liamraystanley/girc.svg)](https://github.com/Liamraystanley/girc/issues)
[![license](https://img.shields.io/github/license/Liamraystanley/girc.svg)](https://raw.githubusercontent.com/Liamraystanley/girc/master/LICENSE)

## Features

- Focuses on simplicity, yet tries to still be flexible
- Only requires standard packages
- Event based triggering/responses
- Documentation is mostly on par

## Installing

    $ go get -u github.com/Liamraystanley/girc

## Examples

See [girc/examples/](https://github.com/Liamraystanley/girc/tree/master/example) for some examples.

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
