FROM golang:1.12-alpine AS builder

WORKDIR /go/src/github.com/tomatool/tomato

COPY . ./

RUN apk add --update make git
RUN make build

# ---

FROM alpine

COPY --from=builder /go/src/github.com/tomatool/tomato/bin/tomato /bin/tomato

ENTRYPOINT  [ "/bin/tomato" ]
CMD         [ "/config.yml" ]
