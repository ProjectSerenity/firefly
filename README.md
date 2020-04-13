# Firefly

Firefly is a microkernel research OS inspired by [Plan 9 from Bell Labs](https://9p.io/plan9/). Firefly's planned name was Serenity, but [SerenityOS](https://github.com/SerenityOS/serenity) got there first. This project is in no way associated with SerenityOS.

The full set of code and build tools for the Firefly kernel is in [`kernel`](/kernel).

Firefly is intended for executing cloud-native containerised server software. As a result, there are no plans to add a graphical user interface, device drivers, or a concept of users. Instead, the priority is to support userland applications on a virtual machine, with strong separation between processes. Firefly will provide a highly stable ABI, with syscalls providing the sole interface between userland processes and the kernel.

Drawing inspiration from Plan 9, _everything is a filesystem_. Overlay filesystems are a fundamental component of Firefly, with network resources, system information, and disk filesystems mounted onto a virtual filesystem. Process namespaces are used to produce independent resource trees for processes, filesystems, and network resources.

## FAQ

### Why make a new OS?

Firefly is primarily an experiment in producing equivalent capabilities for executing cloud-native applications as modern Linux, with a dramatically smaller attack surface and clearer security outcomes. I reckon creating a new OS from scratch will require less work than stripping the irrelevant functionality from Linux.

### Why write the kernel in C?

C was designed for writing kernels (amongst other things) and has a lot of existing code and examples. While other languages will provide benefits over C (such as memory safety or better tooling), they will be lacking in existing code and samples. By defining the interface between userland and the kernel in the syscall ABI, it will be possible to rewrite the kernel in another language at a later point. I reckon building new tooling around C will require less work than writing the kernel in another language.

### Why a microkernel?

The most successful OSs so far have had hybrid monolith kernels. Given Firefly's specific purpose, it will already have a significantly smaller codebase than its competitors. Nevertheless, having the minimum necessary code in the kernel will provide improved security and stability at the expense of performance. If it emerges that the performance loss will be too significant, functionality can be moved to the kernel. I reckon that with modern hardware the performance penalty (which will mainly be in networking) will be acceptable.
