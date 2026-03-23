FROM golang:1.26-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/pusk-platform/pusk/internal/api.Version=${VERSION}" -o pusk ./cmd/pusk/

FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
RUN apk --no-cache add ca-certificates
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
