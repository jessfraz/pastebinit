FROM golang:alpine as builder
MAINTAINER Jessica Frazelle <jess@linux.com>

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN	apk add --no-cache \
	ca-certificates

RUN set -x \
	&& apk add --no-cache --virtual .build-deps \
		git \
		gcc \
		libc-dev \
		libgcc \
	&& go get -v github.com/jessfraz/pastebinit \
	&& cd /go/src/github.com/jessfraz/pastebinit \
	&& CGO_ENABLED=0 go build -a -tags netgo -ldflags '-extldflags "-static"' -o /usr/bin/pastebinit-server ./server \
	&& apk del .build-deps \
	&& rm -rf /go \
	&& echo "Build complete."

FROM scratch

COPY --from=builder /usr/bin/pastebinit-server /usr/bin/pastebinit-server
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs

COPY static /src/static

ENTRYPOINT [ "pastebinit-server" ]
CMD [ "--help" ]
