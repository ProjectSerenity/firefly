//! Provides access to the llvm tools installed through the `llvm-tools-preview` rustup component.

#![deny(missing_docs)]

use std::{borrow::Cow, io, ops::Deref, path::PathBuf, process::Command, string::FromUtf8Error};

/// Allows to retrieve paths of llvm tools.
#[derive(Debug)]
pub struct LlvmTools {
    bin_dir: PathBuf,
}

impl LlvmTools {
    /// Find the directory where the llvm-tools live.
    ///
    /// This only works if the user has the `llvm-tools-preview` rustup component installed.
    pub fn new() -> Result<Self, Error> {
        let sysroot = {
            let mut rustc_cmd = Command::new("rustc");
            rustc_cmd.arg("--print").arg("sysroot");
            let rustc_cmd_string = format!("{:?}", rustc_cmd);
            let out = rustc_cmd
                .output()
                .map_err(|e| Error::CommandExecution(rustc_cmd_string, e))?;
            if !out.status.success() {
                return Err(Error::RetrieveSysroot(String::from_utf8(out.stderr)));
            }

            let sysroot_string =
                String::from_utf8(out.stdout).map_err(Error::SysrootInvalidUtf8)?;
            PathBuf::from(sysroot_string.trim())
        };

        let example_tool_name = exe("llvm-objdump");
        let rustlib = sysroot.join("lib").join("rustlib");
        for entry in rustlib.read_dir().map_err(Error::ReadDirFailed)? {
            let bin_dir = entry.map_err(Error::ReadEntryFailed)?.path().join("bin");
            let tool_path = bin_dir.join(example_tool_name.deref());
            if tool_path.exists() {
                return Ok(Self { bin_dir });
            }
        }

        Err(Error::NotFound)
    }

    /// Returns the path to the specified tool, or None if the tool is not found.
    ///
    /// Use the [`exe`] function to append `*.exe` to the executable name on Windows.
    pub fn tool(&self, tool_name: &str) -> Option<PathBuf> {
        let tool_path = self.bin_dir.join(&tool_name);

        if tool_path.exists() {
            Some(tool_path)
        } else {
            None
        }
    }
}

/// Errors that can occur during the construction of [`LlvmTools`].
#[derive(Debug)]
pub enum Error {
    /// Failed to run a command
    CommandExecution(String, io::Error),
    /// Failed to find out the sysroot by running `rustc --print sysroot`
    RetrieveSysroot(Result<String, FromUtf8Error>),
    /// The sysroot path is not valid utf8
    SysrootInvalidUtf8(FromUtf8Error),
    /// Calling `read_dir` on sysroot directory failed
    ReadDirFailed(io::Error),
    /// Failed to read entry of sysroot directory
    ReadEntryFailed(io::Error),
    /// Could not find llvm-tools component
    ///
    /// Maybe the rustup component `llvm-tools-preview` is missing? Install it through:
    /// `rustup component add llvm-tools-preview`
    NotFound,
}

/// Appends `*.exe` on Windows, returns the original name on other platforms.
pub fn exe(executable_name: &str) -> Cow<str> {
    if cfg!(target_os = "windows") {
        Cow::Owned(format!("{}.exe", executable_name))
    } else {
        Cow::Borrowed(executable_name)
    }
}
