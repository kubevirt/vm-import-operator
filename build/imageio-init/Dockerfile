FROM golang:1.12 AS builder
WORKDIR /src/vm-import-operator/
COPY . .
ENV GOFLAGS=-mod=vendor
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -o /imageio-init tools/imageio-init/main.go

FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

# install controller binary
COPY --from=builder /imageio-init /imageio-init
COPY --from=builder /src/vm-import-operator/tools/imageio-init/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
