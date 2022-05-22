# llvm-tools

Provides access to the llvm tools installed through the `llvm-tools-preview` rustup component.

## Usage

Add the dependency in your `Cargo.toml`:

```toml
[dependencies]
llvm-tools = "0.1"
```

Get the path to the `llvm-objcopy` that is shipped with Rust:

```rust
let llvm_tools = llvm_tools::LlvmTools::new().expect("failed to find llvm-tools");
let objcopy = llvm_tools.tool(&llvm_tools::exe("llvm-objcopy"))
    .expect("llvm-objcopy not found in llvm-tools");
let mut cmd = Command::new(objcopy);
cmd.arg("…");
[…]
```

Note this only works if the user has the `llvm-tools-preview` rustup component installed. Else [`Error::NotFound`](https://docs.rs/llvm-tools/0.1.0/llvm_tools/enum.Error.html#variant.NotFound) is returned.

## License

Licensed under either of

- Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE) or
  http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

at your option.

Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in the work by you, as defined in the Apache-2.0 license, shall be
dual licensed as above, without any additional terms or conditions.
