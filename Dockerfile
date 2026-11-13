# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24 AS build
WORKDIR /src

COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -o /out/api ./cmd/api

FROM alpine:3.24.4
RUN addgroup -S -g 10001 app && \
    adduser -S -D -H -u 10001 -G app app
WORKDIR /app

COPY --from=build /out/api ./api

USER app:app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
