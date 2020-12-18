FROM registry.access.redhat.com/ubi8/go-toolset:1.14.12 AS builder
WORKDIR /opt/app-root/src/vm-import-operator/
COPY . .
ENV GOFLAGS=-mod=vendor
ENV GO111MODULE=on
RUN	CGO_ENABLED=0 GOOS=linux go build -o /opt/app-root/vm-import-controller cmd/manager/main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
ENV CONTROLLER=/usr/local/bin/vm-import-controller \
    USER_UID=1001 \
    USER_NAME=vm-import-controller

# install controller binary
COPY --from=builder /opt/app-root/vm-import-controller ${CONTROLLER}
# Controller needs timezone data for VM validation
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY build/controller/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
