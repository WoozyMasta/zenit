# binaries build
FROM docker.io/golang:1.25-alpine AS build

# hadolint ignore=DL3018
RUN ["apk", "add", "--no-cache", "make", "bash"]
WORKDIR /src
COPY go.mod go.sum ./
RUN ["go", "mod", "download"]
COPY . ./
RUN ["make", "tool-minify", "build"]
RUN echo "zenit:x:1000:1000:zenit:/data:/sbin/nologin" > ./passwd && \
    echo "zenit:x:1000:" > ./group

# create final root fs
FROM scratch AS rootfs

COPY --from=build /src/group /etc/group
COPY --from=build /src/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /src/build/zenit /bin/zenit
WORKDIR /maps

# final binaries image
FROM scratch

USER 1000
WORKDIR /data
ENV PATH=/bin \
    ZENIT_LISTEN_ADDRESS=0.0.0.0:8080 \
    ZENIT_ALLOWED_APPS=MetricZ \
    ZENIT_DB_PATH=/data/zenit.db \
    ZENIT_GEOIP_PATH=/data/zenit.mmdb \
    ZENIT_LOG_LEVEL=info
COPY --from=rootfs --chown=1000:1000 / /
ENTRYPOINT ["/bin/zenit"]
