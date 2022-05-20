FROM golang:1.14 as dev_source
RUN curl -sL https://taskfile.dev/install.sh | sh

FROM alpine:3.11 as dev_exec
RUN apk add --update --no-cache \
    git \
    git-lfs \
    openssh
COPY scripts/askpass.sh /usr/local/bin/askpass.sh
ADD scripts/install_git_crypt.sh install_git_crypt.sh
RUN ./install_git_crypt.sh && rm ./install_git_crypt.sh

FROM dev_source as builder
ADD . /go/src/github.com/telia-oss/github-pr-resource
WORKDIR /go/src/github.com/telia-oss/github-pr-resource
RUN task build

FROM dev_exec as resource
COPY --from=builder /go/src/github.com/telia-oss/github-pr-resource/build /opt/resource
RUN chmod +x /opt/resource/*

FROM resource
LABEL MAINTAINER=telia-oss
