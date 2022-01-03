//! Implements various utilities and data structures used elsewhere in the kernel.
//!
//! ## Bitmaps
//!
//! The [`bitmap`] module can be used to a binary state for an arbitrary number
//! of sequential items efficently. For example, a bitmap could track whether
//! each 4 KiB frame in 2 GiB of physical memory with only 64 KiB of overhead.
//!
//! ## Lazy initialisation
//!
//! The [`Lazy`](lazy::Lazy) type can be used to create an uninitialised value,
//! which is later initialised before use.
//!
//! ## Single initialisation
//!
//! The [`Once`](once::Once) type can be used to create an uninitialised value,
//! which is later initialised exactly once.
//!
//! ## Pretty printing
//!
//! The [`pretty`] module provides helper types for pretty printing units, such
//! as a number of bytes.

pub mod bitmap;
pub mod lazy;
pub mod once;
pub mod pretty;
