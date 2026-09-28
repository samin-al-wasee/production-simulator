FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY main.go telemetry.go redis.go ./
COPY index.html ./
RUN go build -o /out/sample-web .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=build /out/sample-web /usr/local/bin/sample-web

# Run unprivileged: the server binds a high port and writes nothing to disk.
USER 65532:65532

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sample-web"]