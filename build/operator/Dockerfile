FROM golang:1.12 AS builder
WORKDIR /src/vm-import-operator/
COPY . .
ENV GOFLAGS=-mod=vendor
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -o /csv-generator tools/csv-generator/csv-generator.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /vm-import-operator cmd/operator/operator.go

FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/vm-import-operator \
    USER_UID=1001 \
    USER_NAME=vm-import-operator \
    CSV_GENERATOR=/usr/bin/csv-generator

# install operator binary
COPY --from=builder /vm-import-operator ${OPERATOR}
COPY --from=builder /csv-generator ${CSV_GENERATOR}
COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
