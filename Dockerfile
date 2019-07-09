
#build stage
FROM golang:1.12 AS builder
ARG CMDB_TARGET=cmdb_adminserver
WORKDIR /go/src/configcenter
COPY . .
RUN go get -d -v ./...
RUN go install -v ./...

#final stage
FROM centos:centos7
COPY --from=builder /go/bin/configcenter /configcenter
ENTRYPOINT ./configcenter
LABEL Name=configcenter Version=0.0.1
EXPOSE 60000
