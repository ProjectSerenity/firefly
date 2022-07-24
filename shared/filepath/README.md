# Filepath

Filepaths are used in Firefly to describe files and folders. A filepath is a UTF-8 string with the following properties:

1. The path contains at least one code point.
2. The path consists of at least one path element, separated by slashes ('/', '0x2f'), or a single slash.
3. There is at least one printable code point between any two separators.
4. No path element contains a NUL byte ('0x00').
5. No path element exists of exactly one full stop ('.', '0x2e'). This syntax is reserved for future use.
6. No path element exists of exactly two full stops. This syntax is reserved for possible future use.

A filepath that starts with a slash is absolute, whereas one that starts with a path element is relative to the current directory.
