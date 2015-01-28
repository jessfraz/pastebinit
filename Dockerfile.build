FROM golang:latest
MAINTAINER Jessica Frazelle <jess@docker.com>

RUN go get github.com/sourcegraph/syntaxhighlight

EXPOSE 8080

COPY server/ /src

WORKDIR /src

ENTRYPOINT [ "go", "run", "server.go" ]
