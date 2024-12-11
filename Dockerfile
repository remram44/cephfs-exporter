FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS build
RUN apt-get update && apt-get install -yy librados-dev libcephfs-dev && rm -rf /var/lib/apt/lists/*
ARG TARGETARCH
WORKDIR /usr/src/app
COPY *.go go.mod go.sum ./
RUN CGO_ENABLED=1 GOOS=linux GOARCH=$TARGETARCH go build -tags netgo -ldflags -w -o bin/cephfs-exporter-$TARGETARCH ./main.go

FROM debian:bookworm
RUN apt-get update && apt-get install -yy librados2 libcephfs2 && rm -rf /var/lib/apt/lists/*
ARG TARGETARCH
COPY --from=build /usr/src/app/bin/cephfs-exporter-$TARGETARCH /usr/local/bin/cephfs-exporter
CMD ["cephfs-exporter"]
