# Gregal Jocs: minimal image (a single static binary). Build context = repo root:
#   docker build -f deploy/jocs/Dockerfile -t gregal-jocs .
ARG GO_VERSION=1.27
FROM golang:${GO_VERSION} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY jocs ./jocs
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false \
      -ldflags "-X main.version=${VERSION}" -o /out/gregal-jocs ./jocs/cmd/gregal-jocs

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && \
    adduser -D -u 1000 -h /data gregal && install -d -o gregal -g gregal /data
COPY --from=build /out/gregal-jocs /usr/local/bin/gregal-jocs
COPY deploy/jocs/entrypoint.sh /usr/local/bin/jocs-entrypoint
USER gregal
# All state (config, models, accounts, matches) lives in /data: it is the volume.
ENV HOME=/data GREGAL_PERSIST_KEYS=1 GREGAL_ACCOUNTS_DIR=/data/accounts
VOLUME /data
EXPOSE 8197
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8197/jocs/ >/dev/null || exit 1
ENTRYPOINT ["jocs-entrypoint"]
