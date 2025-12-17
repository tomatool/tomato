FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the test app
RUN CGO_ENABLED=0 GOOS=linux go build -o /testapp ./tests/app

# Runtime image
FROM alpine:3.21

RUN apk --no-cache add ca-certificates

COPY --from=builder /testapp /testapp

EXPOSE 8080

CMD ["/testapp"]
