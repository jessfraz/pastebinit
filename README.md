pastebinit
==========

[![Travis CI](https://travis-ci.org/jessfraz/pastebinit.svg?branch=master)](https://travis-ci.org/jessfraz/pastebinit)

Go implementation of pastebinit. Host your own pastebin and post things there. Example file I posted [here](https://paste.j3ss.co/F6CSRR5l).

*Why you ask?* because pastebin.com has ads (booo) & is fugly as eff.

## Installation

#### Binaries

- **darwin** [386](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-darwin-386) / [amd64](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-darwin-amd64)
- **freebsd** [386](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-freebsd-386) / [amd64](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-freebsd-amd64)
- **linux** [386](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-linux-386) / [amd64](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-linux-amd64) / [arm](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-linux-arm) / [arm64](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-linux-arm64)
- **solaris** [amd64](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-solaris-amd64)
- **windows** [386](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-windows-386) / [amd64](https://github.com/jessfraz/pastebinit/releases/download/v0.2.0/pastebinit-windows-amd64)

#### Via Go

```bash
$ go get github.com/jessfraz/pastebinit
```

## Usage

### Client

You need to set `PASTEBINIT_USERNAME` and `PASTEBINIT_PASS` as enviornment variables,
so the client knows how to auth on paste. To change the uri, pass the `-b` flag.

Just like the pastebinit you are used to, this client can read from stdin & input. Heres some examples:

```bash
# pipe to pastebinit
$ docker images | pastebinit -b yoururl.com

# pass a file
$ pastebinit -b yoururl.com server.go
```

### Server

The server can be run in a docker container, via the included dockerfile.
You can use my image on the hub: [jess/pastebinit-server](https://registry.hub.docker.com/u/jess/pastebinit-server/)
or you can build the image yourself via:

```bash
$ git clone git@github.com/jessfraz/pastebinit.git
$ cd pastebinit
$ docker build -i your_name/pastebinit ./server
```

To run the image do, you need to pass the `PASTEBINIT_USERNAME` and `PASTEBINIT_PASS` environment variables to the container.
You can also pass the following options as cli flags to the binary in the container, these are:

- `baseuri, -b`: The uri of the domain you are going to be hosting this on, ex: https://paste.j3ss.co
- `port, -p`: The port to run the app on, defaults to 8080
- `storage, s`: The folder to store your posted pastes in, defaults to `files/`
- `certFile, --cert`: For https servers, path to ssl certificate
- `keyFile, --key`: For https servers, path to ssl key

Example command to run the container:

```bash
# to share the paste volume with your host
$ docker run -d --name=pastebinit --restart=always \
-e PASTEBINIT_USERNAME=your_username -e PASTEBINIT_PASS=your_pass \
-v  /home/jess/pastes:/src/files \
docker_image_name -b https://myserver.com

# to not share the paste volume
$ docker run -d --name=pastebinit --restart=always \
-e PASTEBINIT_USERNAME=your_username -e PASTEBINIT_PASS=your_pass \
docker_image_name -b https://myserver.com

# ssl example
$ docker run -d --name=pastebinit --restart=always \
-e PASTEBINIT_USERNAME=your_username -e PASTEBINIT_PASS=your_pass \
-v /path/to/ssl/stuffs:/ssl \
docker_image_name -b https://myserver.com --cert=/ssl/cert.crt --key=/ssl/key.key
```

Then you are all set! Happy pasteing!
