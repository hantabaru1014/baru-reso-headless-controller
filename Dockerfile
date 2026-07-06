FROM --platform=$BUILDPLATFORM golang:1.25 AS build
ARG TARGETARCH

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

RUN --mount=type=bind,source=.,target=.,ro \
    GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /go/bin/app cmd/server/main.go

# DepotDownloader (self-contained .NET バイナリ) をビルド時に取得して焼き込む.
# 実行時に GitHub Releases へ依存しないようにするため.
FROM debian:12-slim AS depotdownloader
ARG TARGETARCH
ARG DEPOT_DOWNLOADER_VERSION=3.4.0
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget unzip \
    && rm -rf /var/lib/apt/lists/*
RUN case "${TARGETARCH}" in \
      amd64) DD_ARCH=x64 ;; \
      arm64) DD_ARCH=arm64 ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && wget -q -O /tmp/depotdownloader.zip "https://github.com/SteamRE/DepotDownloader/releases/download/DepotDownloader_${DEPOT_DOWNLOADER_VERSION}/DepotDownloader-linux-${DD_ARCH}.zip" \
    && unzip -o /tmp/depotdownloader.zip -d /opt/depotdownloader \
    && chmod +x /opt/depotdownloader/DepotDownloader \
    && rm /tmp/depotdownloader.zip

# runtime: image_builder が git / DepotDownloader / docker CLI を exec する前提の構成.
# - git, ca-certificates: container repo の clone/fetch
# - libicu72: DepotDownloader (.NET) の実行に必要
# - docker CLI + buildx: headless-container の Dockerfile は BuildKit 必須
#   (ビルド自体はマウントされた docker.sock 経由でホストデーモン上で走る)
FROM debian:12-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git libicu72 \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 nonroot \
    && useradd --uid 65532 --gid nonroot --create-home --shell /usr/sbin/nologin nonroot \
    && mkdir -p /data \
    && chown nonroot:nonroot /data

COPY --from=docker:28-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=docker/buildx-bin:0.35.0 /buildx /usr/libexec/docker/cli-plugins/docker-buildx
COPY --from=depotdownloader /opt/depotdownloader /opt/depotdownloader
RUN ln -s /opt/depotdownloader/DepotDownloader /usr/local/bin/DepotDownloader

COPY --from=build /go/bin/app /app

# container repo の checkout / Resonite download 先. compose で volume を割り当てて永続化する.
ENV RESONITE_CONTAINER_REPO_PATH=/data/container-src

USER nonroot
ENTRYPOINT ["/app"]
