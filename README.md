# pastebinit

[![make-all](https://github.com/jessfraz/pastebinit/workflows/make%20all/badge.svg)](https://github.com/jessfraz/pastebinit/actions?query=workflow%3A%22make+all%22)
[![make-image](https://github.com/jessfraz/pastebinit/workflows/make%20image/badge.svg)](https://github.com/jessfraz/pastebinit/actions?query=workflow%3A%22make+image%22)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge)](https://godoc.org/github.com/jessfraz/pastebinit)

Go implementation of pastebinit. Host your own pastebin and post things there. Example file I posted [here](https://paste.j3ss.co/F6CSRR5l).

*Why? You ask..* because pastebin.com has ads (booo) & is fugly as eff.

**Table of Contents**

<!-- toc -->

- [Installation](#installation)
    + [Binaries](#binaries)
    + [Via Go](#via-go)
- [Usage](#usage)
  * [Client](#client)
  * [Server](#server)
    + [Running in a container](#running-in-a-container)

<!-- tocstop -->

## Installation

#### Binaries

For installation instructions from binaries please visit the [Releases Page](https://github.com/jessfraz/pastebinit/releases).

#### Via Go

```console
$ go get github.com/jessfraz/pastebinit
```

## Usage

### Client

You need to set `PASTEBINIT_USERNAME` and `PASTEBINIT_PASSWORD` as environment variables,
so the client knows how to auth on paste. To change the uri, pass the `-b` flag.

Just like the pastebinit you are used to, this client can read from stdin & input. Heres some examples:

```console
# pipe to pastebinit
$ docker images | pastebinit -b yoururl.com

# pass a file
$ pastebinit -b yoururl.com server.go
```

```console
$ pastebinit -h
pastebinit -  Command line paste bin.

Usage: pastebinit <command>

Flags:

  -b, --uri       pastebin base uri (default: https://paste.j3ss.co/)
  -d, --debug     enable debug logging (default: false)
  -p, --password  password (or env var PASTEBINIT_PASSWORD) (default: <none>)
  -u, --username  username (or env var PASTEBINIT_USERNAME)

Commands:

  server   Run the server.
  version  Show the version information.
```

### Server

The server can be run in a docker container, via the included dockerfile.
You can use my image on the hub: [jess/pastebinit](https://registry.hub.docker.com/u/jess/pastebinit/)
or you can build the image yourself via:

```console
$ git clone git@github.com/jessfraz/pastebinit.git
$ cd pastebinit
$ docker build -t your_name/pastebinit .
```

To run the image do, you need to pass the `PASTEBINIT_USERNAME` and `PASTEBINIT_PASSWORD` environment variables to the container.
You can also pass the following options as cli flags to the binary in the container, these are:

```console
$ pastebinit server -h
Usage: pastebinit server [OPTIONS]

Run the server.

Flags:

  --asset-path    Path to assets and templates (default: /src/static)
  -b, --uri       pastebin base uri (default: https://paste.j3ss.co/)
  --cert          path to ssl cert (default: <none>)
  -d, --debug     enable debug logging (default: false)
  --key           path to ssl key (default: <none>)
  -p, --password  password (or env var PASTEBINIT_PASSWORD) (default: <none>)
  --port          port for server to run on (default: 8080)
  -s, --storage   directory to store pastes (default: /etc/pastebinit/files)
  -u, --username  username (or env var PASTEBINIT_USERNAME)
```

#### Running in a container

Example command to run the container:

```console
# to share the paste volume with your host
$ docker run -d \
    --name=pastebinit \
    --restart=always \
    -e PASTEBINIT_USERNAME=your_username \
    -e PASTEBINIT_PASSWORD=your_pass \
    -v  /home/jess/pastes:/src/files \
    jess/pastebinit server \
        -b https://myserver.com

# to not share the paste volume
$ docker run -d \
    --name=pastebinit \
    --restart=always \
    -e PASTEBINIT_USERNAME=your_username \
    -e PASTEBINIT_PASSWORD=your_pass \
    jess/pastebinit server \
        -b https://myserver.com

# ssl example
$ docker run -d 
    --name=pastebinit \
    --restart=always \
    -e PASTEBINIT_USERNAME=your_username \
    -e PASTEBINIT_PASSWORD=your_pass \
    -v /path/to/ssl/stuffs:/ssl \
    jess/pastebinit server \
        -b https://myserver.com --cert=/ssl/cert.crt --key=/ssl/key.key
```

Then you are all set! Happy pasting!
