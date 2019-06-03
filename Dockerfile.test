FROM golang:1.12-alpine AS builder

WORKDIR /go/src/github.com/tomatool/tomato

COPY . ./

RUN apk add --update make git g++
RUN make build-test

# ---

FROM alpine

COPY --from=builder /go/src/github.com/tomatool/tomato/bin/tomato.test /bin/tomato
COPY --from=builder /go/src/github.com/tomatool/tomato/examples/ /

ENTRYPOINT  [ "/bin/tomato" ]
CMD         [ "-test.run=^TestMain$", \
              "-test.coverprofile=/tmp/coverage.out", \
              "/config.yml" ]
