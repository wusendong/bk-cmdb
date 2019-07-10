
#build stage
FROM centos:centos7 AS builder
RUN yum update -y && yum install -y epel-release gcc ca-certificates git wget curl vim less file python-tox python-dev make build-essential nodejs  && \
    rm -f /bin/sh && ln -s /bin/bash /bin/sh && ln -s /usr/bin/nodejs /usr/bin/node
RUN ln -s /usr/bin/nodejs /usr/bin/node
ENV GOPATH=/go PATH=/go/bin:/usr/local/go/bin:${PATH} SHELL=/bin/bash
RUN wget -O - https://storage.googleapis.com/golang/go1.12.7.linux-amd64.tar.gz | tar -xzf - -C /usr/local

ARG BUILD_TARGET
WORKDIR /go/src/configcenter
COPY . .
RUN cd src && make ${BUILD_TARGET} && \
if [ "${BUILD_TARGET}" = "cmdb_webserver" ];\
then\
    make ui;\
fi\
&& make enterprise

#final stage
FROM centos:centos7
ARG BUILD_TARGET
COPY --from=builder /go/src/configcenter/src/bin/enterprise/cmdb /data/

RUN if [ "${BUILD_TARGET}" = "cmdb_adminserver" ];\
then\
    echo /data/cmdb/server/bin/${BUILD_TARGET} '--addrport=$ADDRPORT --config=/etc/cmdb/migrate.conf' > /bin/start.sh;\
else\
    echo /data/cmdb/server/bin/${BUILD_TARGET} '--addrport=$ADDRPORT --regdiscv=$REGDISCV' > /bin/start.sh;\
fi

ENV REGDISCV=127.0.0.1:2181 
ENV ADDRPORT=0.0.0.0:8080
CMD /bin/bash /bin/start.sh

