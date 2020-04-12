FROM ubuntu:latest

RUN apt-get update && \
	apt-get install -y build-essential clang grub2-common

WORKDIR /build

COPY . .

RUN ["/usr/bin/make"]
