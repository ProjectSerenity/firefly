# Packages

## Overview

A Ruse package is a single compilation unit, made of one or more Ruse source files. Packages have a full path, which typically includes a domain name and path, such as `"example.com/foo"` or `"firefly-os.dev/bootloader"`. Each package also has a package name, which is the final component of the package path (`"foo"` and `"bootloader"` in the two previous examples), which must be a valid identifier. Alternatively, a package can have the package name `"main"`, which indicates that it produces an executable binary.

When a package is compiled (using `ruse compile`), it produces an rpkg file, such as `bootloader.rpkg`. This contains all of the constants and executable code of the package, plus the type information for the package and its dependencies.

A package can import other packages, using their full path. By default, this introduces the package's namespace into the file that imports it, bound to an identifier with the package name. Alternatively, a different identifier can be provided. This is demonstrated below:

```
(package foo)

(import "firefly-os.dev/bootloader")              ; Import package "firefly-os.dev/bootloader", making its scope available as `bootloader`.
(import (bootloader2 "example.com/bootloader"))   ; Import package "example.com/bootloader" as `bootloader2`.
```

When accessing objects in another package, they must be exported symbols. A symbol is exported by using a capital letter as the first letter of its name. For example, the constant `name` is not exported but `Name` is.
