# Filesystem

Firefly's approach to filesystems is inspired by Plan 9, but adapted for stateless server workloads. A newly-created filesystem is an empty logical entity, with the root `/`. Other filesystem layers can then be mounted onto the filesystem, including layers backed by storage media and virtual layers providing access to the kernel. The list of common filesystem layers includes:

- `FastBlobStorage`: a temporary blob storage area, backed by main memory.
- `BulkBlobStorage`: a temporary blob storage area, backed by bulk storage, such as a hard drive.
- `TAR`: a filesystem layer defined in a TAR archive file.
- `Network`: an interface with the kernel for accessing the network.
- `Bind`: mounts one part of a filesystem onto another.
- `WriteBlocker`: a virtual layer that does not accept file creation.

For example, a typical OCI container might have its filesystem created by starting with a fresh filesystem, then mounting each layer of its image using `TAR` layers, mounting `/tmp` using a `FastBlobStorage` layer, then mounting a `Network` layer onto `/net`.

## Filesystem layers

When layers are mounted onto the filesystem, they form an overlay filesystem. Opening existing files works by iterating through the mounted layers (in the order in which they were mounted) until a file with the matching filename is found. If such a file is found, it is opened, otherwise an error is returned. Creating new files works similarly, but only the layer first mounted in the relevant area of the filesystem is tried. If the layer creates the file, it is returned, otherwise an error is returned. If for an area of the filesystem file creation is not desired, a `WriteBlocker` layer can be mounted before mounting other layers, which might allow file creation. For example, TCP support can be disabled by mounting a `WriteBlocker` layer on `/net/tcp` before mounting the `Network` layer.

### `FastBlobStorage` layers

The `FastBlobStorage` layer functions similarly to `tmpfs` on Linux. A newly-mounted `FastBlobStorage` layer is empty, allowing the creation of new files. The capacity of the layer is set at its creation. If there is insufficient space in memory to accept data written to the layer, the write will fail, rather than paging data to bulk storage.

### `BulkBlobStorage` layers

The `BulkBlobStorage` layer functions similarly to `tmpfs` on Linux. A newly-mounted `BulkBlobStorage` layer is empty, allowing the creation of new files. The capacity of the layer is set at its creation. If there is insufficient space on the storage medium to accept data written to the layer, the write will fail.

### `TAR` layers

The `TAR` layer creates a layer matching the data described by a TAR archive. The archive is specified using a filename, pointing to a TAR archive file elsewhere on the filesystem. The layer is read-only, so data cannot be written back to the TAR file underlying the layer. As described in the section on filesystem permissions below, Firefly ignores the user and permissions data in the TAR archive, treating all files as readable and executable. `TAR` layers are designed to allow easy execution of OCI containers, whose filesystem layers are defined in TAR archives. Note that `TAR` layers do not perform decompression, so compressed image layer types, such as `application/vnd.oci.image.layer.v1.tar+gzip` must be decompressed before being used to create `TAR` layers.

### `Network` layers

The `Network` layer is used to access the network. When the layer is created, a reference to the underlying network namespace is specified. See the documentation for the network stack for more details.

### `Bind` layers

The `Bind` layer functions similarly to binds in Plan 9 or symlinks in POSIX. A `Bind` layer is created by specifying a filesystem namespace and a filepath in that namespace. Any actions that affect the layer are proxied to the host. This can be used either to make one part of a filesystem visible from another (similar to a symlink) or to make part of one filesystem visible from another filesystem. For example, a process can be created with no network namespace, but access to a pre-existing connection by mounting a `Bind` layer specifying the filepath of the connection in the host namespace, onto the new process's filesystem.

### `WriteBlocker` layers

The `WriteBlocker` layer contains no files and does not allow the creation of new files. Since reads pass through a layer but writes do not, this can be used to prevent a part of the filesystem from accepting writes.

## Filesystem permissions

A general-purpose OS uses a storage medium as the foundation of the filesystem, adding virtual filesystems on top. All files are visible to all processes, so a permissions system is used to manage access. Firefly is designed to execute stateless containerised workloads. As a result, resources are rarely shared between execution contexts. This means there is no need to restrict access within a container, as all resources within a container are equally trusted. As containers are stateless, there is no way to write to the filesystem, other than to temporary storage, which can be deleted when the namespace in which it resides is deleted.
