FROM golang:1.12-alpine3.10

RUN apk add --no-cache \
    git gcc musl-dev

RUN export GO111MODULE=on && \
    go get -v github.com/millerlogic/smallprox/cmd/smallprox@v1.1


FROM alpine:3.10

RUN apk add --no-cache \
    ca-certificates

COPY --from=0 /go/bin/smallprox /usr/local/bin/smallprox

CMD ["smallprox", "-v", "-addr=:8080"]
