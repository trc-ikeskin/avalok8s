# syntax=docker/dockerfile:1

####### build stage ########

FROM golang:1.24 AS builder

# produce statically linked binary without runtime deps
ARG CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build ./...

####### run stage ########

FROM alpine:latest

WORKDIR /app/

RUN apk add --no-cache ca-certificates

COPY --from=builder /src/server .

EXPOSE 8080

ENTRYPOINT ["./server"]
