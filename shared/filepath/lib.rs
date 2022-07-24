// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Helper functionality for processing Firefly filepaths.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![forbid(unsafe_code)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

use alloc::string::{String, ToString};
use alloc::vec::Vec;

/// The separator used in file paths.
///
pub const SEPARATOR: char = '/';

/// An error describing an invalid filepath.
///
/// A filepath is valid provided all of the following rules hold:
///
/// 1. The path contains at least one code point.
/// 2. The path consists of at least one path element, separated by slashes (`/`, `0x2f`), or a single slash.
/// 3. There is at least one printable code point between any two separators.
/// 4. No path element contains a NUL byte (`0x00`).
/// 5. No path element exists of exactly one full stop (`.`, `0x2e`). This syntax is reserved for possible future use.
/// 6. No path element exists of exactly two full stops. This syntax is reserved for possible future use.
///
#[derive(Debug, PartialEq, Eq)]
pub enum InvalidFilepath {
    /// The empty string is not a valid filepath.
    EmptyPath,

    /// A separator was followed by another separator.
    AdjacentSeparators,

    /// An illegal character was found in a path element.
    IllegalCharacter(char),

    /// An illegal path element was found.
    IllegalPathElement(&'static str),
}

/// Returns a result indicating whether the given filepath is valid.
///
/// A filepath is valid provided all of the following rules hold:
///
/// 1. The path contains at least one code point.
/// 2. The path consists of at least one path element, separated by slashes (`/`, `0x2f`), or a single slash.
/// 3. There is at least one printable code point between any two separators.
/// 4. No path element contains a NUL byte (`0x00`).
/// 5. No path element exists of exactly one full stop (`.`, `0x2e`). This syntax is reserved for possible future use.
/// 6. No path element exists of exactly two full stops. This syntax is reserved for possible future use.
///
pub fn validate(path: &str) -> Result<(), InvalidFilepath> {
    if path.is_empty() {
        // 1. The path contains at least one code point.
        return Err(InvalidFilepath::EmptyPath);
    } else if path.len() == 1 && path.starts_with(SEPARATOR) {
        // 2. The path consists of at least one path element,
        // separated by slashes (`/`, `0x2f`), or a single slash.
        return Ok(());
    }

    const DOT: char = '.';
    const DOTDOT: &str = "..";

    // Track whether a path separator is allowed.
    let mut last_sep = None;
    let mut allow_sep = true;
    for (i, c) in path.chars().enumerate() {
        match c {
            '\0' => {
                // 4. No path element contains a NUL byte (`0x00`).
                return Err(InvalidFilepath::IllegalCharacter('\0'));
            }
            SEPARATOR => {
                if !allow_sep {
                    // 3. There is at least one printable code point between any two separators.
                    return Err(InvalidFilepath::AdjacentSeparators);
                }

                // Check that the previous path element
                // was valid.
                let start = if let Some(j) = last_sep { j + 1 } else { 0 };

                let last_element = &path[start..i];
                if last_element.len() == 1 && last_element.starts_with(DOT) {
                    // 5. No path element exists of exactly one full stop (`.`, `0x2e`).
                    // This syntax is reserved for possible future use.
                    return Err(InvalidFilepath::IllegalPathElement("."));
                }

                if last_element.len() == 2 && last_element.starts_with(DOTDOT) {
                    // 6. No path element exists of exactly two full stops.
                    // This syntax is reserved for possible future use.
                    return Err(InvalidFilepath::IllegalPathElement(".."));
                }

                last_sep = Some(i);
                allow_sep = false;
            }
            _ => {
                // Now that we've seen a path element,
                // a separator is allowed.
                allow_sep = true;
            }
        }
    }

    // If the path ends in a path element, then we need
    // to check that it's valid.
    if allow_sep {
        // Check that the previous path element
        // was valid.
        let start = if let Some(j) = last_sep { j + 1 } else { 0 };

        let last_element = &path[start..path.len()];
        if last_element.len() == 1 && last_element.starts_with(DOT) {
            // 5. No path element exists of exactly one full stop (`.`, `0x2e`).
            // This syntax is reserved for possible future use.
            return Err(InvalidFilepath::IllegalPathElement("."));
        }

        if last_element.len() == 2 && last_element.starts_with(DOTDOT) {
            // 6. No path element exists of exactly two full stops.
            // This syntax is reserved for possible future use.
            return Err(InvalidFilepath::IllegalPathElement(".."));
        }
    }

    Ok(())
}

/// Returns whether the given filepath is valid.
///
/// A filepath is valid provided all of the following rules hold:
///
/// 1. The path contains at least one code point.
/// 2. The path consists of at least one path element, separated by slashes (`/`, `0x2f`), or a single slash.
/// 3. There is at least one printable code point between any two separators.
/// 4. No path element contains a NUL byte (`0x00`).
/// 5. No path element exists of exactly one full stop (`.`, `0x2e`). This syntax is reserved for possible future use.
/// 6. No path element exists of exactly two full stops. This syntax is reserved for possible future use.
///
pub fn is_valid(path: &str) -> bool {
    validate(path).is_ok()
}

/// Make `path` absolute, relative to `current`.
///
/// If `path` is already absolute (if it has a leading
/// slash), then it is returned unmodified. Both `path`
/// and `current` are assumed to be valid, according to
/// [`validate`]. However, if `path` is the empty string
/// (which is not a valid filepath), `make_absolute`
/// returns `current`, rather than panicking.
///
pub fn absolute(current: &str, path: &str) -> String {
    if path.is_empty() {
        current.to_string()
    } else if path.starts_with(SEPARATOR) {
        path.to_string()
    } else if current.ends_with(SEPARATOR) {
        let mut out = String::with_capacity(current.len() + path.len());
        out.push_str(current);
        out.push_str(path);
        out
    } else {
        let mut out = String::with_capacity(current.len() + 1 + path.len());
        out.push_str(current);
        out.push(SEPARATOR);
        out.push_str(path);
        out
    }
}

/// Returns the final path element in `path`.
///
/// If `path` ends in a slash, then the path element
/// before that slash is returned. Otherwise, the
/// path element after the final slash is returned.
///
/// If `path` is an invalid filepath, or if it
/// consists of just a slash, `None` is returned.
///
pub fn base_name(path: &str) -> Option<&str> {
    let mut path = path;
    if path.len() == 1 && path.starts_with(SEPARATOR) || !is_valid(path) {
        return None;
    }

    // Ignore any trailing separator.
    if path.ends_with(SEPARATOR) {
        path = &path[0..(path.len() - 1)];
    }

    // Find the last separator (if any).
    if let Some(i) = path.rfind(SEPARATOR) {
        path = &path[(i + 1)..path.len()];
    }

    Some(path)
}

/// Returns the parent directory that contains `path`.
///
/// If `path` ends in a slash, then it is ignored.
/// The string before the final slash (ignoring any
/// trailing slash) is returned.
///
/// If `path` is an invalid filepath, or if it
/// consists of just a slash, or if it contains
/// no slashes, `None` is returned.
///
pub fn parent_directory(path: &str) -> Option<&str> {
    let mut path = path;
    if path.len() == 1 && path.starts_with(SEPARATOR) || !is_valid(path) {
        return None;
    }

    // Ignore any trailing separator.
    if path.ends_with(SEPARATOR) {
        path = &path[0..(path.len() - 1)];
    }

    // Find the last separator (if any).
    match path.rfind(SEPARATOR) {
        Some(i) => {
            // If the parent directory is the root
            // directory, we include the "trailing"
            // slash.
            if i == 0 {
                path = &path[0..=i];
            } else {
                path = &path[0..i];
            }
        }
        None => return None,
    }

    Some(path)
}

/// Returns whether `path` is absolute.
///
/// `is_absolute` does not guarantee that `path`
/// is valid. If `path` is the empty string (which
/// is not valid), then `is_absolute` returns
/// `false`.
///
#[inline]
pub fn is_absolute(path: &str) -> bool {
    !path.is_empty() && path.starts_with(SEPARATOR)
}

/// Joins a sequence of path segments into a
/// single path.
///
/// The segments provided (each of which may begin
/// or end with a separator) are joined together
/// using separators. Provided all segments are
/// valid filepaths (according to [`validate`],
/// the resulting filepath will be valid.
///
pub fn join(segments: &[&str]) -> Result<String, InvalidFilepath> {
    if segments.is_empty() {
        return Err(InvalidFilepath::EmptyPath);
    }

    // Special-case a single segment of just a
    // slash, since it will otherwise be removed
    // by the code that drops a trailing slash.
    if segments.len() == 1 && segments[0] == "/" {
        return Ok("/".to_string());
    }

    // Check that the inputs are all valid and
    // determine the length of the final path.
    let mut length = segments.len() - 1; // Number of separators.
    for (i, segment) in segments.iter().enumerate() {
        // Ignore any sole separators after the
        // first segment.
        if i > 0 && segment.len() == 1 && segment.starts_with(SEPARATOR) {
            continue;
        }

        validate(segment)?;
        length += segment.len();

        // Ignore any leading slash, except for
        // in the first segment, which is kept
        // to keep the resulting path absolute.
        if i > 0 && segment.starts_with(SEPARATOR) {
            length -= 1;
        }

        if segment.ends_with(SEPARATOR) {
            length -= 1;
        }
    }

    let mut path = String::with_capacity(length);
    for (i, segment) in segments.iter().enumerate() {
        if i > 0 && segment.len() == 1 && segment.starts_with(SEPARATOR) {
            continue;
        }

        if i > 0 {
            path.push(SEPARATOR);
        }

        let mut start = 0;
        let mut end = segment.len();

        // Ignore any leading slash, except for
        // in the first segment, which is kept
        // to keep the resulting path absolute.
        if i > 0 && segment.starts_with(SEPARATOR) {
            start += 1;
        }

        if segment.ends_with(SEPARATOR) {
            end -= 1;
        }

        path.push_str(&segment[start..end]);
    }

    Ok(path)
}

/// Returns the set of path elements in `path`.
///
/// If `path` is an absolute path, the first path
/// element will be the leading slash. If `path`
/// includes at least one path element and it ends
/// with a slash, that trailing slash is ignored.
///
/// # Panics
///
/// If `path` is not valid (according to [`validate`]),
/// `path_elements` will panic.
///
pub fn path_elements(path: &str) -> Vec<&str> {
    validate(path).expect("valid path");

    let path = if path.ends_with(SEPARATOR) {
        &path[0..(path.len() - 1)]
    } else {
        path
    };

    // Work out how many path elements
    // to include.
    let num_elements = path.chars().filter(|&c| c == SEPARATOR).count();
    let mut elements = Vec::with_capacity(num_elements);
    let mut last_sep = 0;
    for (i, c) in path.chars().enumerate() {
        if c == SEPARATOR {
            if i == 0 {
                elements.push(&path[0..1]);
            } else {
                elements.push(&path[last_sep..i]);
            }

            last_sep = i + 1;
        }
    }

    elements.push(&path[last_sep..path.len()]);

    elements
}

#[cfg(test)]
mod test {
    use super::*;
    use alloc::vec;

    #[test]
    fn test_is_valid() {
        macro_rules! valid_path {
            ($path:literal) => {
                assert!(is_valid($path), $path);
                assert_eq!(validate($path), Ok(()), $path);
            };
        }

        macro_rules! invalid_path {
            ($path:literal, $err:expr) => {
                assert!(!is_valid($path), $path);
                assert_eq!(validate($path), Err($err), $path);
            };
        }

        valid_path!("...");
        valid_path!(".../foo");
        valid_path!("foo/...");
        valid_path!("a");
        valid_path!("a/");
        valid_path!("a/b");
        valid_path!("a/b/");
        valid_path!("a/b/c");
        valid_path!("a/b/c/");
        valid_path!("foo");
        valid_path!("foo/");
        valid_path!("foo/bar");
        valid_path!("foo/bar/");
        valid_path!("foo/bar/baz");
        valid_path!("foo/bar/baz/");
        valid_path!(".a");
        valid_path!(".a/");
        valid_path!("a/.b");
        valid_path!("a/.b/");
        valid_path!("a/.b/c");
        valid_path!("a/.b/c/");
        valid_path!("/...");
        valid_path!("/.../foo");
        valid_path!("/foo/...");
        valid_path!("/");
        valid_path!("/a");
        valid_path!("/a/");
        valid_path!("/a/b");
        valid_path!("/a/b/");
        valid_path!("/a/b/c");
        valid_path!("/a/b/c/");
        valid_path!("/foo");
        valid_path!("/foo/");
        valid_path!("/foo/bar");
        valid_path!("/foo/bar/");
        valid_path!("/foo/bar/baz");
        valid_path!("/foo/bar/baz/");
        valid_path!("/.a");
        valid_path!("/.a/");
        valid_path!("/a/.b");
        valid_path!("/a/.b/");
        valid_path!("/a/.b/c");
        valid_path!("/a/.b/c/");

        // 1. The path contains at least one code point.
        invalid_path!("", InvalidFilepath::EmptyPath);

        // 2. The path consists of at least one path element, separated by slashes (`/`, `0x2f`).
        // Can't break this rule without breaking rule 1.

        // 3. There is at least one printable code point between any two separators.
        invalid_path!("a//", InvalidFilepath::AdjacentSeparators);
        invalid_path!("a/b//", InvalidFilepath::AdjacentSeparators);
        invalid_path!("//", InvalidFilepath::AdjacentSeparators);
        invalid_path!("/a//", InvalidFilepath::AdjacentSeparators);
        invalid_path!("/a/b//", InvalidFilepath::AdjacentSeparators);

        // 4. No path element contains a NUL byte (`0x00`).
        invalid_path!("\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("a\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("a/\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("a/b\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("/\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("/a\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("/a/\0", InvalidFilepath::IllegalCharacter('\0'));
        invalid_path!("/a/b\0", InvalidFilepath::IllegalCharacter('\0'));

        // 5. No path element exists of exactly one full stop (`.`, `0x2e`).
        // This syntax is reserved for possible future use.
        invalid_path!(".", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("./a", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("a/.", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("a/./", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("a/b/.", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("a/b/./", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("/.", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("/./", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("/a/.", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("/a/b/.", InvalidFilepath::IllegalPathElement("."));
        invalid_path!("/a/b/./", InvalidFilepath::IllegalPathElement("."));

        // 6. No path element exists of exactly two full stops.
        // This syntax is reserved for possible future use.
        invalid_path!("..", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("../a", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("a/..", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("a/../", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("a/b/..", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("a/b/../", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("/..", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("/../", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("/a/..", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("/a/b/..", InvalidFilepath::IllegalPathElement(".."));
        invalid_path!("/a/b/../", InvalidFilepath::IllegalPathElement(".."));
    }

    #[test]
    fn test_absolute() {
        assert_eq!(absolute("/", ""), "/");
        assert_eq!(absolute("/foo", ""), "/foo");
        assert_eq!(absolute("/", "a"), "/a");
        assert_eq!(absolute("/", "/a"), "/a");
        assert_eq!(absolute("/foo", "bar"), "/foo/bar");
        assert_eq!(absolute("/foo/", "bar"), "/foo/bar");
        assert_eq!(absolute("/foo/", "bar/"), "/foo/bar/");
        assert_eq!(absolute("/foo", "/bar"), "/bar");
        assert_eq!(absolute("/foo", "/bar/"), "/bar/");
        assert_eq!(absolute("/foo/", "/bar"), "/bar");
        assert_eq!(absolute("/foo/", "/bar/"), "/bar/");
        assert_eq!(absolute("/a/b", "foo/bar"), "/a/b/foo/bar");
        assert_eq!(absolute("/a/b", "foo/bar/"), "/a/b/foo/bar/");
    }

    #[test]
    fn test_base_name() {
        macro_rules! valid_base_name {
            ($path:literal, $want:literal) => {
                assert_eq!(base_name($path), Some($want), $path);
            };
        }

        macro_rules! invalid_base_name {
            ($path:literal) => {
                assert_eq!(base_name($path), None, $path);
            };
        }

        valid_base_name!("a", "a");
        valid_base_name!("a/", "a");
        valid_base_name!("a/b/c", "c");
        valid_base_name!("a/b/c/", "c");
        valid_base_name!("a/b/.c", ".c");
        valid_base_name!("a/b/c.", "c.");
        valid_base_name!("a/b/c.d", "c.d");
        valid_base_name!("/a", "a");
        valid_base_name!("/a/", "a");
        valid_base_name!("/a/b/c", "c");
        valid_base_name!("/a/b/c/", "c");
        valid_base_name!("/a/b/.c", ".c");
        valid_base_name!("/a/b/c.", "c.");
        valid_base_name!("/a/b/c.d", "c.d");

        invalid_base_name!("");
        invalid_base_name!("/");
        invalid_base_name!("//");
    }

    #[test]
    fn test_parent_directory() {
        macro_rules! valid_parent_directory {
            ($path:literal, $want:literal) => {
                assert_eq!(parent_directory($path), Some($want), $path);
            };
        }

        macro_rules! invalid_parent_directory {
            ($path:literal) => {
                assert_eq!(parent_directory($path), None, $path);
            };
        }

        valid_parent_directory!("a/b/c", "a/b");
        valid_parent_directory!("a/b/c/", "a/b");
        valid_parent_directory!("a/b/.c", "a/b");
        valid_parent_directory!("a/b/c.", "a/b");
        valid_parent_directory!("a/b/c.d", "a/b");
        valid_parent_directory!("/a", "/");
        valid_parent_directory!("/a/", "/");
        valid_parent_directory!("/a/b/c", "/a/b");
        valid_parent_directory!("/a/b/c/", "/a/b");
        valid_parent_directory!("/a/b/.c", "/a/b");
        valid_parent_directory!("/a/b/c.", "/a/b");
        valid_parent_directory!("/a/b/c.d", "/a/b");

        invalid_parent_directory!("");
        invalid_parent_directory!("/");
        invalid_parent_directory!("//");
        invalid_parent_directory!("a");
        invalid_parent_directory!("a/");
        invalid_parent_directory!("foo");
        invalid_parent_directory!("foo/");
    }

    #[test]
    fn test_is_absolute() {
        assert!(is_absolute("/"));
        assert!(is_absolute("/foo"));
        assert!(is_absolute("/foo/"));
        assert!(is_absolute("/a/b"));

        assert!(!is_absolute(""));
        assert!(!is_absolute("foo"));
        assert!(!is_absolute("foo/"));
        assert!(!is_absolute("foo/bar"));
        assert!(!is_absolute("a/b"));
        assert!(!is_absolute("a/b/"));
        assert!(!is_absolute("a/b/c"));
    }

    #[test]
    fn test_join() {
        macro_rules! valid_join {
            ($segments:expr, $want:literal) => {
                assert_eq!(join($segments), Ok($want.to_string()), $want);
            };
        }

        macro_rules! invalid_join {
            ($segments:expr, $err:expr) => {
                assert_eq!(join($segments), Err($err));
            };
        }

        valid_join!(&["a"], "a");
        valid_join!(&["a/b"], "a/b");
        valid_join!(&["a/b/c"], "a/b/c");
        valid_join!(&["a", "b"], "a/b");
        valid_join!(&["a/b", "c"], "a/b/c");
        valid_join!(&["a/b", "/", "c"], "a/b/c");
        valid_join!(&["a", "b", "c"], "a/b/c");
        valid_join!(&["/"], "/");
        valid_join!(&["/a"], "/a");
        valid_join!(&["/a/b"], "/a/b");
        valid_join!(&["/a/b/c"], "/a/b/c");
        valid_join!(&["/a", "b"], "/a/b");
        valid_join!(&["/a/b", "c"], "/a/b/c");
        valid_join!(&["/a", "b", "c"], "/a/b/c");
        valid_join!(&["/a", "/", "b", "c"], "/a/b/c");
        valid_join!(&["/", "a", "b", "c"], "/a/b/c");

        invalid_join!(&[], InvalidFilepath::EmptyPath);
        invalid_join!(&[""], InvalidFilepath::EmptyPath);
        invalid_join!(&["", "a"], InvalidFilepath::EmptyPath);
        invalid_join!(&["a", ""], InvalidFilepath::EmptyPath);
        invalid_join!(&["a", "", "b"], InvalidFilepath::EmptyPath);
    }

    #[test]
    fn test_path_elements() {
        macro_rules! split {
            ($path:literal, $($want:literal),+) => {
                assert_eq!(path_elements($path), vec![$($want),+]);
            };
        }

        split!("a", "a");
        split!("a/b", "a", "b");
        split!("/a", "/", "a");
        split!("/a/b", "/", "a", "b");
        split!("/a/b/", "/", "a", "b");
    }
}
