FROM ubuntu:latest

RUN apt-get update && \
	apt-get install -y build-essential clang nasm grub2-common curl tar

RUN curl https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz | tar -xzf - -C /usr/local

# Get Rust
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y

ENV GOBIN="/tmp/gobin"
ENV PATH="${PATH}:/usr/local/go/bin:${GOBIN}:/root/.cargo/bin"

# Get cargo-xbuild, switch to nightly channel.
RUN rustup override add nightly
RUN rustup component add rust-src
RUN rustup component add llvm-tools-preview
RUN cargo install cargo-xbuild
RUN cargo install bootimage

WORKDIR /build/kernel

CMD ["cargo", "bootimage", "--release"]
