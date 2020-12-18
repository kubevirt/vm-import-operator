FROM registry.access.redhat.com/ubi8/go-toolset:1.14.12 AS builder
WORKDIR /opt/app-root/src/vm-import-operator/
COPY . .
ENV GOFLAGS=-mod=vendor
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -o /opt/app-root/csv-generator tools/csv-generator/csv-generator.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /opt/app-root/vm-import-operator cmd/operator/operator.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/vm-import-operator \
    USER_UID=1001 \
    USER_NAME=vm-import-operator \
    CSV_GENERATOR=/usr/bin/csv-generator

# install operator binary
COPY --from=builder /opt/app-root/vm-import-operator ${OPERATOR}
COPY --from=builder /opt/app-root/csv-generator ${CSV_GENERATOR}
COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
