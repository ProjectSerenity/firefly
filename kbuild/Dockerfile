FROM ubuntu:20.04

# Install build tools.
# - binutils for objcopy
# - xorriso
# - grub-pc-bin for grub-mkrescue
# - nasm
# - gcc
# - libc-dev (for the intermediate build)
# - curl (to fetch Go below)
# - ca-certificates (to fetch Go below)
RUN apt-get update && \
    apt-get install -y --no-install-recommends binutils xorriso grub-pc-bin nasm gcc libc-dev curl ca-certificates && \
    update-ca-certificates

# Install Go.
# We get this directly, rather than through apt,
# so we can get more recent versions.
# We use -L to follow redirects.
RUN curl -L https://golang.org/dl/go1.15.6.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV PATH="/usr/local/go/bin:${PATH}"

# Add and build kbuild.
# We do the go.* first so we don't have to
# re-fetch dependencies unnecessarily.
ADD go.* /kbuild/
RUN cd /kbuild && go mod download
ADD . /kbuild/
RUN cd /kbuild && go build -o /kbuild/kbuild

# Switch to Go 1.8 for the build.
RUN rm -rf /usr/local/go && curl -L https://golang.org/dl/go1.8.7.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# Prepare for the build.
ENV GOPATH=/kernel
WORKDIR /kernel/src/github.com/ProjectSerenity/firefly/kernel
CMD ["/kbuild/kbuild"]
