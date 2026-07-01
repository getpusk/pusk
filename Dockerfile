FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/pusk-platform/pusk/internal/api.Version=${VERSION}" -o pusk ./cmd/pusk/

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk --no-cache upgrade && apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/pusk .
COPY --from=builder /build/web/static ./web/static
RUN mkdir -p data
EXPOSE 8443
VOLUME /app/data
ENV PUSK_ADDR=:8443
RUN addgroup -S pusk && adduser -S pusk -G pusk && chown -R pusk:pusk /app
USER pusk
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8443/api/health || exit 1
CMD ["./pusk"]
