FROM registry.access.redhat.com/ubi8/go-toolset:1.14.12 AS builder
WORKDIR /opt/app-root/src/vm-import-operator/
COPY . .
ENV GOFLAGS=-mod=vendor
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -o /opt/app-root/imageio-init tools/imageio-init/main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

# install controller binary
COPY --from=builder /opt/app-root/imageio-init /imageio-init
COPY --from=builder /opt/app-root/src/vm-import-operator/tools/imageio-init/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
