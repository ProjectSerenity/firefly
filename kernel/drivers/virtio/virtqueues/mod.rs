// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains modules for different VirtIO [Virtqueues](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-230005).
//!
//! ## Split Virtqueues
//!
//! The [split] module provides the split [`Virtqueue`](split::Virtqueue), which can be
//! used to exchange buffers with a VirtIO device.
//!
//! ## Packed Virtqueues
//!
//! VirtIO [packed Virtqueue](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-610007)
//! are not yet supported.

// TODO(#9): Implement packed virtqueues.

pub mod split;
