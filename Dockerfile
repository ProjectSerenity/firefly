FROM ubuntu:latest

RUN apt-get update && \
	apt-get install -y build-essential clang nasm grub2-common curl tar

RUN curl https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz | tar -xzf - -C /usr/local

ENV GOBIN="/tmp/gobin"
ENV PATH="${PATH}:/usr/local/go/bin:${GOBIN}"

WORKDIR /build

COPY . .

RUN ["/usr/bin/make"]
