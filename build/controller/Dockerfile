FROM golang:1.13 AS builder
WORKDIR /src/vm-import-operator/
COPY . .
ENV GOFLAGS=-mod=vendor
ENV GO111MODULE=on
RUN	CGO_ENABLED=0 GOOS=linux go build -o /vm-import-controller cmd/manager/main.go

FROM registry.access.redhat.com/ubi7/ubi-minimal:latest
ENV CONTROLLER=/usr/local/bin/vm-import-controller \
    USER_UID=1001 \
    USER_NAME=vm-import-controller

# install controller binary
COPY --from=builder /vm-import-controller ${CONTROLLER}
# Controller needs timezone data for VM validation
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY build/controller/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
