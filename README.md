# Firefly

Firefly is a research OS inspired by [Plan 9 from Bell Labs](https://9p.io/plan9/). Firefly's planned name was Serenity, but [SerenityOS](https://github.com/SerenityOS/serenity) got there first. This project is in no way associated with SerenityOS.

This repository consists of:

- the Firefly kernel in [`kernel`](/kernel)

- the Firefly kernel builder in [`kbuild`](/kbuild)

Firefly is intended for executing cloud-native containerised server software. As a result, there are no plans to add a graphical user interface, device drivers, or a concept of users. Instead, the priority is to support userland applications on a virtual machine, with strong separation between processes. Firefly will provide a highly stable ABI, with syscalls providing the sole interface between userland processes and the kernel.

Drawing inspiration from Plan 9, _everything is a filesystem_. Overlay filesystems are a fundamental component of Firefly, with network resources, system information, and disk filesystems mounted onto a virtual filesystem. Process namespaces are used to produce independent resource trees for processes, filesystems, and network resources.

# Building Firefly

Firefly is built using [`kbuild`](/kbuild). Either use the dependencies described in kbuild, or use Docker. Building the kernel can then be performed using [`./kernel/build`](/kernel/build) from the `kernel` directory.

# Running Firefly

Firefly can be run with QEMU using the [`./kernel/run`](/kernel/run) helper script from the `kernel` directory.
Firefly has the following dependencies:

- An Intel x86_64 processor, of Ivy Bridge generation or later
- At least 128 MiB of RAM

# Documentation

Further documentation is in [`./docs`](/docs).

## FAQ

### Why make a new OS?

Firefly is primarily an experiment in producing equivalent capabilities for executing cloud-native applications as modern Linux, with a dramatically smaller attack surface and clearer security outcomes. I reckon creating a new OS from scratch will require less work than stripping the irrelevant functionality from Linux.

### Why write the kernel in Go?

While Go is designed for userspace programs, a little patching of the runtime makes it suitable for kernel code. The advantage to using Go is that it is type-safe and memory-safe, and shares a lot of code with the userland programs it runs.
