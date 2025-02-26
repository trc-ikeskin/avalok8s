# syntax=docker/dockerfile:1

####### backend build stage ########

FROM docker.io/library/golang:1.24 AS backend-build

# produce statically linked binary without runtime deps
ARG CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -o ./bin/ ./...

####### frontend build stage ########

FROM --platform=$BUILDPLATFORM docker.io/library/node:latest AS frontend-build

WORKDIR /src

COPY ui/package*.json ./

RUN npm install

COPY ui/ .

RUN NODE_OPTIONS=--max_old_space_size=2048 npm run build

####### run stage ########

FROM alpine:latest

ENV PORT=8080

WORKDIR /app/

RUN apk add --no-cache ca-certificates

COPY --from=backend-build /src/bin/server .
COPY --from=frontend-build src/dist ./ui/dist

EXPOSE ${PORT}

ENTRYPOINT ["./server"]
