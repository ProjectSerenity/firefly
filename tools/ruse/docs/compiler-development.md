# Compiler development

## Compiler pipeline

The Ruse compiler toolchain follows a fairly straightforward pipeline, starting with the compiler (`cmd/compile`):

- `lexer` is used to scan the source code into `token.Token` instances.
- `parser` is used to parse the token stream into `ast` types, such as `ast.Expression`, `ast.File`, and `ast.Package`.
- `types` is used to type-check the syntax tree, populating type information into a `types.Info`.
- `compiler` is used to assemble and encode any assmebly code (see [`assembly.md`](./assembly.md) and compile Ruse code to `ssafir.Value` instances.
- `compiler` is used again to allocate registers to the abstract instruction of Ruse functions.
- `compiler` is used one last time to lower abstract instructions of Ruse functions to architecture-specific concrete instructions.
- `rpkg` is used to encode the compiled functions, constants, and configuration to an rpkg file.

For this, the compiler needs to be supplied with the rpkg files for those packages directly imported by the package being compiled. That is, the rpkg files for indirect dependencies are not needed by the compiler.

Next, the linker (`cmd/link`) is used to compbine a set of packages into an executable binary:

- `rpkg` is used to parse each rpkg file, reconstructing each package as a `compiler.Package`.
- The linker builds the different program sections and constructs the symbol table.
- The binary encoder (currently just `binary/elf`) is used to construct and write the executable binary.

Unlike the compiler, the linker requires the rpkg files for all dependencies of the main package, including indirect dependencies.
