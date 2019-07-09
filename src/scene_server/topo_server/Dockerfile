FROM golang:1.12 AS BUILDER

RUN go build -o cmdb_toposerver

CMD [ "cmdb_adminserver" "--addrport=0.0.0.0:8080" "--logtostderr=true" "--v=4"]
