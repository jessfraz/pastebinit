FROM progrium/busybox
MAINTAINER Jessica Frazelle <jess@docker.com>

COPY server/static /src/static

ADD https://jesss.s3.amazonaws.com/binaries/pastebinit-server /usr/local/bin/pastebinit-server

RUN chmod +x /usr/local/bin/pastebinit-server

ENTRYPOINT [ "/usr/local/bin/pastebinit-server" ]
