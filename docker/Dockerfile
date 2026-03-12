# Building stage
FROM docker.m.daocloud.io/golang:1.24

RUN apk add --update make

WORKDIR /go/src/github.com/guyuxiang/gin-apiserver

COPY . /go/src/github.com/guyuxiang/gin-apiserver

ENV CGO_ENABLED 0
ENV GOOS linux
ENV GO111MODULE=on

ENV TIMEZONE "Asia/Shanghai"

RUN make src.build
RUN mv build/pkg/cmd/gin-apiserver/gin-apiserver /go/bin

# Production stage
FROM alpine:3.9
WORKDIR /go/bin

LABEL maintainer="guyuxiang <guyuxiang@qq.com>"

# copy the go binaries from the building stage
COPY --from=0 /go/bin /go/bin

COPY etc/gin-apiserver.yaml /etc/gin-apiserver/gin-apiserver.yaml

EXPOSE 80

ENTRYPOINT ["./gin-apiserver"]
