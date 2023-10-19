# Executable configuration

## Executable binary layout

There are two main tools for configuring the layout of the executable binary, both of which are controlled in code. By default, the linker uses a base address of `0x20_0000` (2 MiB). Loadable program sections start at this address, ensuring that each new section starts in a new 4 kiB page. For example, a small binary would place the code section (containing executable code, marked read-execute) at address `0x20_0000` and the strings section at the next page (`0x20_1000`). The first way this can be configured is by changing the base address. This can be done by using the `base-address` annotation on the package statement, specifying a new base address. Note that this annotation can only be used with the package statement for package main.

```
'(base-address 0x10_0000)
(package main)
```

The second way this can be configured is by controlling the set of program sections. The default sections are defined in the `"sections"` package in the standard library. These are:

- `sections.Code` defines the `code` section, which has read-execute permissions. If functions (including assembly functions) do not specify a section, the linker places them in the code section.
- `sections.Strings` defines the `strings` section, which has read permissions. If string constants (including anonymous strings) do not specify a section, the linker places them in the strings section.
- `sections.RPKGs` defines the `rpkgs` section, which has read permissions. If provenance data is not disabled in the linker, the linker places it in the rpkgs section.

Additional sections can be declared by assigning them to a constant. For example, to create a section called `bootstrap` at the fixed address `0x10_0000`, with all permissions, the following syntax could be used:

```
(let bootstrap (section
	(name "bootstrap")
	(fixed-address 0x10_0000)
	(permissions rwx)))
```

Additionally, the set of program sections used, and their order, can be specified using another annotation on the main package statement. This references sections by their identifier. For example, to include only the standard sections, but to reverse their order, the following syntax could be used:

```
'(sections sections.RPKGs sections.Strings sections.Code)
(package main)

(import "sections")
```

Finally, symbols can be assigned to a non-default section by using a `'(section)` annotation on the corresponding `(let)`, `(func)`, or `(asm-func)` statement. For example, additional string data could be included in the rpkgs section as follows:

```
(import "sections")

'(section sections.RPKGs)
(let foo "Extra provenance text")
```

Note that any sections omitted from the `'(sections)` annotation will not be included in the binary, even if they contain important data. For example, omitting `sections.Code` will exclude any executable code that does not specify another section that is included in the annotation.
