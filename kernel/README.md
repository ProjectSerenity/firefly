# Firefly kernel

The Firely kernel is a microkernel. It's still under development and doesn't yet have an ABI.

## Building

The kernel can be built either in a container, or locally. The [`Dockerfile`](Dockerfile)
describes the dependencies needed to build the kernel.

To build in a container, use the [`docker-build`](docker-build) helper script.

```bash
$ ./docker-build
```

To build locally, use [`make`](https://www.gnu.org/software/make/).

```bash
$ make
```

## Running

Firefly is designed to be used in a virtual machine, with `make` producing a standard
multiboot kernel image. The kernel can be run in QEMU using the [`run-qemu`](run-qemu)
helper script.

```bash
$ ./run-qemu
```
