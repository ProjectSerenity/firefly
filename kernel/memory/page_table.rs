// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

use crate::constants::PHYSICAL_MEMORY_OFFSET;
use crate::{
    InvalidPhysAddr, PhysAddr, PhysFrame, PhysFrameAllocator, PhysFrameSize, VirtAddr, VirtPage,
    VirtPageSize,
};
use bitflags::bitflags;
use x86_64::instructions::tlb;

// The 51st bit of a physical frame address in
// a page table entry is reserved and must be
// unset. This mask unsets all bits outside the
// valid physical address space, plus bits 51
// and bits 11 and below, which are always 0
// anyway, due to frame alignment.
//
const ADDR_MASK: u64 = 0x0007_ffff_ffff_f000;

/// Represents a single entry in a page table.
///
#[derive(Clone)]
#[repr(transparent)]
pub struct PageTableEntry {
    entry: u64,
}

impl PageTableEntry {
    /// Returns a new, empty page table entry.
    ///
    #[inline]
    pub const fn new() -> Self {
        PageTableEntry { entry: 0 }
    }

    /// Clears this entry.
    ///
    #[inline]
    pub fn zero(&mut self) {
        self.entry = 0;
    }

    /// Returns whether the entry is marked as present.
    ///
    #[inline]
    pub const fn is_present(&self) -> bool {
        PageTableFlags::from_bits_truncate(self.entry).present()
    }

    /// Returns the flags for this entry.
    ///
    #[inline]
    pub const fn flags(&self) -> PageTableFlags {
        PageTableFlags::from_bits_truncate(self.entry)
    }

    /// Returns the physical address in this entry.
    /// Note that this address will always have the
    /// least significant 12 bits unset.
    ///
    #[inline]
    pub const fn addr(&self) -> PhysAddr {
        PhysAddr::new(self.entry as usize & PhysFrameSize::Size4KiB.start_mask())
    }

    /// Returns the child page table referenced by this
    /// entry.
    ///
    /// # Safety
    ///
    /// This dereferences the address in this entry,
    /// which is unsafe.
    ///
    #[inline]
    pub unsafe fn page_table(&self) -> Result<PageTable, InvalidPhysAddr> {
        let addr = self.addr();
        PageTable::at(addr)
    }

    /// Returns the child page table referenced by this
    /// entry.
    ///
    /// # Safety
    ///
    /// This dereferences the address in this entry,
    /// which is unsafe.
    ///
    #[inline]
    unsafe fn page_table_at(&self, physmem_offset: VirtAddr) -> PageTable {
        let addr = self.addr();
        PageTable::at_offset(addr, physmem_offset)
    }

    /// Returns the physical frame referenced by this
    /// entry.
    ///
    #[inline]
    pub fn phys_frame(&self, frame_size: PhysFrameSize) -> Result<PhysFrame, InvalidPhysAddr> {
        let addr = self.addr();
        PhysFrame::from_start_address(addr, frame_size)
    }

    /// Sets the entry's flags.
    ///
    #[inline]
    pub fn set_flags(&mut self, flags: PageTableFlags) {
        self.entry = flags.bits() | self.addr().as_usize() as u64;
    }

    /// Sets the entry's physical address to the given
    /// frame. If the top bit of the frame's start
    /// address is set, `set_frame` returns it as an
    /// `InvalidPhysAddr`.
    ///
    #[inline]
    pub fn set_frame(&mut self, frame: PhysFrame) -> Result<(), InvalidPhysAddr> {
        let start_addr = frame.start_address().as_usize() as u64;
        if start_addr & ADDR_MASK != start_addr {
            Err(InvalidPhysAddr(start_addr as usize))
        } else {
            self.entry = self.flags().bits() | start_addr;
            Ok(())
        }
    }

    /// Sets the entry's physical address to `frame` and
    /// the flags to `flags`. If the top bit of the frame's
    /// start address is set, `set_frame` returns it as an
    /// `InvalidPhysAddr`.
    ///
    #[inline]
    pub fn set_frame_flags(
        &mut self,
        frame: PhysFrame,
        flags: PageTableFlags,
    ) -> Result<(), InvalidPhysAddr> {
        let start_addr = frame.start_address().as_usize() as u64;
        if start_addr & ADDR_MASK != start_addr {
            Err(InvalidPhysAddr(start_addr as usize))
        } else {
            self.entry = flags.bits() | start_addr;
            Ok(())
        }
    }

    /// Creates a new page table for this entry if one
    /// does not already exist.
    ///
    /// The flags for this entry will have the bits in
    /// `flags` set. If any other flag bits in this entry
    /// are set, but are not set in `flags`, those bits
    /// will not be unset.
    ///
    fn ensure_page_table_at<A>(
        &mut self,
        flags: PageTableFlags,
        allocator: &mut A,
        offset: VirtAddr,
    ) -> Result<PageTable, PageMappingError>
    where
        A: PhysFrameAllocator + ?Sized,
    {
        // If there's already a page table present, we
        // just update the flags and return it.
        if self.is_present() || self.addr() != PhysAddr::zero() {
            let current = self.flags();
            if !flags.is_empty() && !current.contains(flags) {
                self.set_flags(flags | current);
            }

            return Ok(unsafe { self.page_table_at(offset) });
        }

        // Allocate a physical frame.
        if let Some(frame) = allocator.allocate_phys_frame(PhysFrameSize::Size4KiB) {
            // If we get a frame we can't use, we return an error.
            // In an ideal world, we would also deallocate the
            // frame, but that would require the allocator to
            // always support deallocation. This is not true
            // in the kernel's initial moments.
            //
            // This error can only occur if we have used roughly
            // 2048 TiB of physical memory.
            self.set_frame_flags(frame, flags)
                .map_err(|InvalidPhysAddr(addr)| PageMappingError::InvalidPhysAddr(addr))?;
        } else {
            return Err(PageMappingError::PageTableAllocationFailed);
        }

        let mut page_table = unsafe { self.page_table_at(offset) };
        page_table.zero();

        Ok(page_table)
    }
}

bitflags! {
    /// The flags in a page table entry.
    ///
    pub struct PageTableFlags: u64 {
        /// Indicates that the referenced page table or
        /// physical frame is mapped and usable.
        const PRESENT = 1 << 0;

        /// Indicates that the referenced page tables
        /// or physical frame is writable. If this bit
        /// is unset when referencing a page table,
        /// none of the memory referenced by that page
        /// table (and any child tables) is writable,
        /// irrespective of those page tables' flags.
        const WRITABLE = 1 << 1;

        /// Indicates that accesses from ring 3 are
        /// allowed. If unset, only rings 2 and below
        /// can access the memory.
        const USER_ACCESSIBLE = 1 << 2;

        /// Indicates that the caching behaviour for
        /// this memory is write-through. Otherwise,
        /// it is write-back.
        const WRITE_THROUGH = 1 << 3;

        /// Indicates that the memory should not be
        /// cached.
        const NO_CACHE = 1 << 4;

        /// Indicates that the referenced page table or
        /// physical frame has been read since this bit
        /// was last unset.
        const ACCESSED = 1 << 5;

        /// Indicates that the referenced page table or
        /// physical frame has been written since this
        /// bit was last unset.
        const DIRTY = 1 << 6;

        /// Indicates that this references a huge
        /// physical frame, rather than a page table.
        const HUGE_PAGE = 1 << 7;

        /// Indicates that this mapping is not flushed
        /// from the TLB when an address space change
        /// takes place.
        const GLOBAL = 1 << 8;

        /// Indicates that the referenced memory cannot
        /// be used for instruction fetches and is not
        /// executable.
        const NO_EXECUTE = 1 << 63;
    }
}

impl PageTableFlags {
    /// Returns whether the `PRESENT` flag is set.
    ///
    pub const fn present(&self) -> bool {
        self.contains(Self::PRESENT)
    }

    /// Returns whether the `PRESENT` flag is unset.
    ///
    pub const fn absent(&self) -> bool {
        !self.contains(Self::PRESENT)
    }

    /// Returns whether the `WRITABLE` flag is set.
    ///
    pub const fn writable(&self) -> bool {
        self.contains(Self::WRITABLE)
    }

    /// Returns whether the `WRITABLE` flag is unset.
    ///
    pub const fn read_only(&self) -> bool {
        !self.contains(Self::WRITABLE)
    }

    /// Returns whether the `HUGE_PAGE` flag is set.
    ///
    pub const fn huge(&self) -> bool {
        self.contains(Self::HUGE_PAGE)
    }

    /// Returns whether the `HUGE_PAGE` flag is unset.
    ///
    pub const fn small(&self) -> bool {
        !self.contains(Self::HUGE_PAGE)
    }

    /// Returns whether the `GLOBAL` flag is set.
    ///
    pub const fn global(&self) -> bool {
        self.contains(Self::GLOBAL)
    }

    /// Returns whether the `GLOBAL` flag is unset.
    ///
    pub const fn local(&self) -> bool {
        !self.contains(Self::GLOBAL)
    }

    /// Returns whether the `NO_EXECUTE` flag is set.
    ///
    pub const fn no_execute(&self) -> bool {
        self.contains(Self::NO_EXECUTE)
    }

    /// Returns whether the `NO_EXECUTE` flag is unset.
    ///
    pub const fn executable(&self) -> bool {
        !self.contains(Self::NO_EXECUTE)
    }
}

/// The number of entries in a page table.
///
const NUM_ENTRIES: usize = 512;

/// The mask for indices into a page table.
///
const ENTRY_MASK: usize = NUM_ENTRIES - 1;

/// Provides access to a page table and its contents.
///
pub struct PageTable<'entries> {
    // Virtual address at which all physical memory is
    // already mapped. We use this to access page tables
    // (which use physical addresses) via virtual memory.
    physmem_offset: VirtAddr,

    // A reference/pointer to the actual page table.
    table: &'entries mut [PageTableEntry; NUM_ENTRIES],
}

impl<'entries> PageTable<'entries> {
    /// Creates a page table referring to the page table data
    /// at the given address.
    ///
    /// If `addr` is not frame-aligned, then `at` will return
    /// [`InvalidPhysAddr`].
    ///
    /// # Safety
    ///
    /// This dereferences `addr`, which is unsafe.
    ///
    pub unsafe fn at(addr: PhysAddr) -> Result<Self, InvalidPhysAddr> {
        if !addr.is_aligned(PhysFrameSize::Size4KiB.bytes()) {
            Err(InvalidPhysAddr(addr.as_usize()))
        } else {
            let physmem_offset = PHYSICAL_MEMORY_OFFSET;
            Ok(Self::at_offset(addr, physmem_offset))
        }
    }

    /// Creates a page table from the page table data at the
    /// given address, using the provided virtual memory
    /// offset at which physical memory is mapped.
    ///
    /// # Note
    ///
    /// This is designed for testing, where an alternative
    /// offset can be used, such as for emulating physical
    /// memory in userspace by providing an offset of `0`.
    ///
    /// In use cases outside testing, use [`PageTable::at`]
    /// instead.
    ///
    /// # Safety
    ///
    /// This dereferences `addr`, which is unsafe.
    ///
    pub unsafe fn at_offset(addr: PhysAddr, physmem_offset: VirtAddr) -> Self {
        let table_addr = physmem_offset
            .checked_add(addr.as_usize())
            .expect("invalid physical address");
        let table_ptr = table_addr.as_usize() as *mut [PageTableEntry; NUM_ENTRIES];
        let table = &mut *table_ptr;

        PageTable {
            physmem_offset,
            table,
        }
    }

    /// Clears the page table, setting every entry to zero.
    ///
    /// Note that this does nothing to any child page tables
    /// or physical frames currently referenced by this page
    /// table.
    ///
    pub fn zero(&mut self) {
        for entry in self.table.iter_mut() {
            entry.zero();
        }
    }

    /// Iterate through the entries in this page table.
    ///
    pub fn iter(&self) -> impl Iterator<Item = &PageTableEntry> {
        self.table.iter()
    }

    /// Translate the given virtual address to a physical
    /// address, according to the page tables.
    ///
    pub fn translate(&self, addr: VirtAddr) -> PageMapping {
        let level4_entry = &self.table[level4_index(addr)];
        let level4_flags = level4_entry.flags();
        if level4_flags.absent() {
            return PageMapping::NotMapped;
        } else if level4_flags.huge() {
            return PageMapping::InvalidLevel4PageTable;
        }

        let level3_table = unsafe { level4_entry.page_table_at(self.physmem_offset) };
        let level3_entry = &level3_table.table[level3_index(addr)];
        let level3_flags = level3_entry.flags();
        if level3_flags.absent() {
            return PageMapping::NotMapped;
        } else if level3_flags.huge() {
            let size = PhysFrameSize::Size1GiB;
            let frame = match level3_entry.phys_frame(size) {
                Ok(frame) => frame,
                Err(addr) => return PageMapping::InvalidPageTableAddr(addr.0),
            };

            let offset = addr.as_usize() & (size.bytes() - 1);
            let addr = frame.start_address() + offset;
            let flags = level3_flags;
            return PageMapping::Mapping { frame, addr, flags };
        }

        let level2_table = unsafe { level3_entry.page_table_at(self.physmem_offset) };
        let level2_entry = &level2_table.table[level2_index(addr)];
        let level2_flags = level2_entry.flags();
        if level2_flags.absent() {
            return PageMapping::NotMapped;
        } else if level2_flags.huge() {
            let size = PhysFrameSize::Size2MiB;
            let frame = match level2_entry.phys_frame(size) {
                Ok(frame) => frame,
                Err(addr) => return PageMapping::InvalidPageTableAddr(addr.0),
            };

            let offset = addr.as_usize() & (size.bytes() - 1);
            let addr = frame.start_address() + offset;
            let flags = level2_flags;
            return PageMapping::Mapping { frame, addr, flags };
        }

        let level1_table = unsafe { level2_entry.page_table_at(self.physmem_offset) };
        let level1_entry = &level1_table.table[level1_index(addr)];
        let level1_flags = level1_entry.flags();
        if level1_flags.absent() {
            return PageMapping::NotMapped;
        }

        let size = PhysFrameSize::Size4KiB;
        let frame = match level1_entry.phys_frame(size) {
            Ok(frame) => frame,
            Err(addr) => return PageMapping::InvalidPageTableAddr(addr.0),
        };

        let offset = addr.as_usize() & (size.bytes() - 1);
        let addr = frame.start_address() + offset;
        let flags = level1_flags;
        PageMapping::Mapping { frame, addr, flags }
    }

    /// Translate the given virtual address to a physical
    /// address, according to the page tables.
    ///
    pub fn translate_addr(&self, addr: VirtAddr) -> Option<PhysAddr> {
        if let PageMapping::Mapping { addr, .. } = self.translate(addr) {
            Some(addr)
        } else {
            None
        }
    }

    /// Translate the given virtual page to a physical
    /// frame, according to the page tables.
    ///
    pub fn translate_page(&self, page: VirtPage) -> Option<PhysFrame> {
        if let PageMapping::Mapping { frame, .. } = self.translate(page.start_address()) {
            Some(frame)
        } else {
            None
        }
    }

    /// Create a new mapping in the page table.
    ///
    /// The `allocator` may be called to allocate physical
    /// memory for any new page tables that are created.
    ///
    /// # Panics
    ///
    /// If the virtual page and the physical frame are not
    /// of identical size, `map` will panic.
    ///
    /// # Safety
    ///
    /// Making a page mapping is unsafe, as the caller must
    /// ensure that any other mappings to `frame` are not
    /// used concurrently with changes to `page`. This is
    /// one of the kernel's biggest responsibilities.
    ///
    pub unsafe fn map<A>(
        &mut self,
        page: VirtPage,
        frame: PhysFrame,
        flags: PageTableFlags,
        allocator: &mut A,
    ) -> Result<PageMappingChange, PageMappingError>
    where
        A: PhysFrameAllocator + ?Sized,
    {
        assert_eq!(page.size(), frame.size());

        let size = page.size();
        let addr = page.start_address();
        let offset = self.physmem_offset;
        let parent_table_flags =
            PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::USER_ACCESSIBLE;

        let level4_entry = &mut self.table[level4_index(addr)];
        if level4_entry.flags().huge() {
            // Strictly speaking, this is an invalid entry,
            // but this error is near enough.
            return Err(PageMappingError::LargerParentMappingExists);
        }

        let level3_table =
            level4_entry.ensure_page_table_at(parent_table_flags, allocator, offset)?;
        let level3_entry = &mut level3_table.table[level3_index(addr)];
        if size == VirtPageSize::Size1GiB {
            if level3_entry.is_present() || level3_entry.addr() != PhysAddr::zero() {
                return Err(PageMappingError::PageAlreadyMapped(
                    PhysFrame::containing_address(level3_entry.addr(), frame.size()),
                ));
            }

            level3_entry
                .set_frame_flags(frame, flags | PageTableFlags::HUGE_PAGE)
                .map_err(|InvalidPhysAddr(addr)| PageMappingError::InvalidPhysAddr(addr))?;
            return Ok(PageMappingChange::new(page));
        }

        if level3_entry.flags().huge() {
            return Err(PageMappingError::LargerParentMappingExists);
        }

        let level2_table =
            level3_entry.ensure_page_table_at(parent_table_flags, allocator, offset)?;
        let level2_entry = &mut level2_table.table[level2_index(addr)];
        if size == VirtPageSize::Size2MiB {
            if level2_entry.is_present() || level2_entry.addr() != PhysAddr::zero() {
                return Err(PageMappingError::PageAlreadyMapped(
                    PhysFrame::containing_address(level2_entry.addr(), frame.size()),
                ));
            }

            level2_entry
                .set_frame_flags(frame, flags | PageTableFlags::HUGE_PAGE)
                .map_err(|InvalidPhysAddr(addr)| PageMappingError::InvalidPhysAddr(addr))?;
            return Ok(PageMappingChange::new(page));
        }

        if level2_entry.flags().huge() {
            return Err(PageMappingError::LargerParentMappingExists);
        }

        let level1_table =
            level2_entry.ensure_page_table_at(parent_table_flags, allocator, offset)?;
        let level1_entry = &mut level1_table.table[level1_index(addr)];
        if level1_entry.is_present() || level1_entry.addr() != PhysAddr::zero() {
            return Err(PageMappingError::PageAlreadyMapped(
                PhysFrame::containing_address(level1_entry.addr(), frame.size()),
            ));
        }

        level1_entry
            .set_frame_flags(frame, flags & !PageTableFlags::HUGE_PAGE)
            .map_err(|InvalidPhysAddr(addr)| PageMappingError::InvalidPhysAddr(addr))?;
        Ok(PageMappingChange::new(page))
    }

    /// Change the page table flags for an existing mapping
    /// in the page table.
    ///
    /// # Safety
    ///
    /// Changing a page mapping is unsafe, as it may invalidate
    /// assumptions made by the compiler about whether data
    /// can be modified.
    ///
    pub unsafe fn change_flags(
        &mut self,
        page: VirtPage,
        flags: PageTableFlags,
    ) -> Result<PageMappingChange, PageRemappingError> {
        let size = page.size();
        let addr = page.start_address();
        let offset = self.physmem_offset;

        let level4_entry = &mut self.table[level4_index(addr)];
        let level4_flags = level4_entry.flags();
        if level4_flags.absent() {
            return Err(PageRemappingError::PageNotMapped);
        } else if level4_flags.huge() {
            // Strictly speaking, this is an invalid entry,
            // but this error is near enough.
            return Err(PageRemappingError::LargerParentMappingExists);
        }

        let level3_table = level4_entry.page_table_at(offset);
        let level3_entry = &mut level3_table.table[level3_index(addr)];
        let level3_flags = level3_entry.flags();
        if size == VirtPageSize::Size1GiB {
            if level3_flags.absent() || level3_flags.small() {
                return Err(PageRemappingError::PageNotMapped);
            }

            level3_entry.set_flags(flags | PageTableFlags::HUGE_PAGE);

            return Ok(PageMappingChange::new(page));
        }

        if level3_flags.huge() {
            return Err(PageRemappingError::LargerParentMappingExists);
        }

        let level2_table = level3_entry.page_table_at(offset);
        let level2_entry = &mut level2_table.table[level2_index(addr)];
        let level2_flags = level2_entry.flags();
        if size == VirtPageSize::Size2MiB {
            if level2_flags.absent() || level2_flags.small() {
                return Err(PageRemappingError::PageNotMapped);
            }

            level2_entry.set_flags(flags | PageTableFlags::HUGE_PAGE);

            return Ok(PageMappingChange::new(page));
        }

        if level2_flags.huge() {
            return Err(PageRemappingError::LargerParentMappingExists);
        }

        let level1_table = level2_entry.page_table_at(offset);
        let level1_entry = &mut level1_table.table[level1_index(addr)];
        let level1_flags = level1_entry.flags();
        if level1_flags.absent() || level1_flags.huge() {
            return Err(PageRemappingError::PageNotMapped);
        }

        level1_entry.set_flags(flags & !PageTableFlags::HUGE_PAGE);

        Ok(PageMappingChange::new(page))
    }

    /// Remove a mapping from the page table.
    ///
    /// The previous physical frame is returned. Note that
    /// no page tables are allocated or deallocated.
    ///
    /// # Safety
    ///
    /// Removing a page mapping is unsafe, as the caller must
    /// ensure that the virtual address range being unmapped
    /// is no longer in use.
    ///
    pub unsafe fn unmap(
        &mut self,
        page: VirtPage,
    ) -> Result<(PhysFrame, PageMappingChange), PageUnmappingError> {
        let size = page.size();
        let addr = page.start_address();
        let offset = self.physmem_offset;

        let level4_entry = &mut self.table[level4_index(addr)];
        let level4_flags = level4_entry.flags();
        if level4_flags.absent() {
            return Err(PageUnmappingError::PageNotMapped);
        } else if level4_flags.huge() {
            // Strictly speaking, this is an invalid entry,
            // but this error is near enough.
            return Err(PageUnmappingError::LargerParentMappingExists);
        }

        let level3_table = level4_entry.page_table_at(offset);
        let level3_entry = &mut level3_table.table[level3_index(addr)];
        let level3_flags = level3_entry.flags();
        if size == VirtPageSize::Size1GiB {
            if level3_flags.absent() || level3_flags.small() {
                return Err(PageUnmappingError::PageNotMapped);
            }

            let addr = level3_entry.addr();
            let frame = PhysFrame::from_start_address(addr, PhysFrameSize::Size1GiB)
                .map_err(|InvalidPhysAddr(addr)| PageUnmappingError::InvalidPhysAddr(addr))?;
            level3_entry.zero();

            return Ok((frame, PageMappingChange::new(page)));
        }

        if level3_flags.huge() {
            return Err(PageUnmappingError::LargerParentMappingExists);
        }

        let level2_table = level3_entry.page_table_at(offset);
        let level2_entry = &mut level2_table.table[level2_index(addr)];
        let level2_flags = level2_entry.flags();
        if size == VirtPageSize::Size2MiB {
            if level2_flags.absent() || level2_flags.small() {
                return Err(PageUnmappingError::PageNotMapped);
            }

            let addr = level2_entry.addr();
            let frame = PhysFrame::from_start_address(addr, PhysFrameSize::Size2MiB)
                .map_err(|InvalidPhysAddr(addr)| PageUnmappingError::InvalidPhysAddr(addr))?;
            level2_entry.zero();

            return Ok((frame, PageMappingChange::new(page)));
        }

        if level2_flags.huge() {
            return Err(PageUnmappingError::LargerParentMappingExists);
        }

        let level1_table = level2_entry.page_table_at(offset);
        let level1_entry = &mut level1_table.table[level1_index(addr)];
        let level1_flags = level1_entry.flags();
        if level1_flags.absent() || level1_flags.huge() {
            return Err(PageUnmappingError::PageNotMapped);
        }

        let addr = level1_entry.addr();
        let frame = PhysFrame::from_start_address(addr, PhysFrameSize::Size4KiB)
            .map_err(|InvalidPhysAddr(addr)| PageUnmappingError::InvalidPhysAddr(addr))?;
        level1_entry.zero();

        Ok((frame, PageMappingChange::new(page)))
    }
}

/// Describes the result of translating a virtual address
/// to a physical address using a set of page tables.
///
#[derive(Debug, PartialEq)]
pub enum PageMapping {
    /// A valid mapping.
    Mapping {
        /// The frame in which the physical translated address
        /// resides.
        frame: PhysFrame,
        /// The translated physical address.
        addr: PhysAddr,
        /// The page table flags for the translated physical
        /// frame.
        flags: PageTableFlags,
    },
    /// An invalid mapping, due to the [`PRESENT`](PageTableFlags::PRESENT)
    /// flag being unset.
    NotMapped,
    /// An invalid mapping, due to an incorrect page table
    /// physical address.
    InvalidPageTableAddr(usize),
    /// An invalid mapping, due to the level-4 page table
    /// having the [`HUGE_PAGE`](PageTableFlags::HUGE_PAGE)
    /// bit set.
    InvalidLevel4PageTable,
}

/// Describes a page mapping that has been changed. This
/// must either be flushed to update the [TLB](https://en.wikipedia.org/wiki/Translation_lookaside_buffer),
/// or ignored. If the change is ignored, the CPU may not
/// recognise the page mapping.
///
#[derive(Debug)]
#[must_use = "Page mapping changes must either flush the TLB or be ignored explicitly."]
pub struct PageMappingChange(VirtPage);

impl PageMappingChange {
    fn new(page: VirtPage) -> Self {
        PageMappingChange(page)
    }

    /// Flush the relevant portion of the TLB for this
    /// page mapping.
    ///
    #[inline]
    pub fn flush(self) {
        tlb::flush(self.0.start_address().as_x86_64());
    }

    /// Ignore the page mapping change.
    ///
    /// This may result in unexpected behaviour, such as
    /// the CPU ignoring the mapping change for some time.
    ///
    #[inline]
    pub fn ignore(self) {}
}

/// Describes an error encountered while trying to make a
/// new page mapping.
///
#[derive(Debug, PartialEq)]
pub enum PageMappingError {
    /// Failed to allocate a new physical memory frame
    /// for use as a new page table.
    PageTableAllocationFailed,
    /// Tried to create a new page mapping within a
    /// larger existing mapping, such as a 4 KiB page
    /// inside a 2 MiB page.
    LargerParentMappingExists,
    /// This virtual page is already mapped to a physical
    /// memory frame. The frame that is already mapped is
    /// returned.
    PageAlreadyMapped(PhysFrame),
    /// The provided physical frame cannot be used in a
    /// page table entry.
    InvalidPhysAddr(usize),
}

/// Describes an error encountered while trying to change
/// the flags on an existing page mapping.
///
#[derive(Debug, PartialEq)]
pub enum PageRemappingError {
    /// Tried to change a page mapping within a larger
    /// mapping, such as a 4 KiB page inside a 2 MiB
    /// page.
    LargerParentMappingExists,
    /// This virtual page is not mapped.
    PageNotMapped,
}

/// Describes an error encountered while trying to remove
/// a page mapping.
///
#[derive(Debug, PartialEq)]
pub enum PageUnmappingError {
    /// Tried to remove a page mapping within a larger
    /// mapping, such as a 4 KiB page inside a 2 MiB
    /// page.
    LargerParentMappingExists,
    /// This virtual page is not mapped.
    PageNotMapped,
    /// The current mapping was to an invalid physical
    /// frame address.
    InvalidPhysAddr(usize),
}

/// Returns the index into the level 4 page table for the
/// given virtual address.
///
const fn level4_index(addr: VirtAddr) -> usize {
    (addr.as_usize() >> 12 >> 9 >> 9 >> 9) & ENTRY_MASK
}

/// Returns the index into the level 3 page table for the
/// given virtual address.
///
const fn level3_index(addr: VirtAddr) -> usize {
    (addr.as_usize() >> 12 >> 9 >> 9) & ENTRY_MASK
}

/// Returns the index into the level 2 page table for the
/// given virtual address.
///
const fn level2_index(addr: VirtAddr) -> usize {
    (addr.as_usize() >> 12 >> 9) & ENTRY_MASK
}

/// Returns the index into the level 1 page table for the
/// given virtual address.
///
const fn level1_index(addr: VirtAddr) -> usize {
    (addr.as_usize() >> 12) & ENTRY_MASK
}

#[cfg(test)]
mod test {
    extern crate std;
    use super::*;
    use std::boxed::Box;
    use std::vec;
    use std::vec::Vec;
    use x86_64::structures::paging;
    use x86_64::structures::paging::{Mapper, Translate};

    #[test]
    fn test_page_table_indices() {
        let addr = VirtAddr::new(0xffff_8234_5678_9abc);
        assert_eq!(level4_index(addr), 260);
        assert_eq!(level3_index(addr), 209);
        assert_eq!(level2_index(addr), 179);
        assert_eq!(level1_index(addr), 393);
    }

    // This includes a byte array the same size as
    // a page table, aligned to frame boundaries.
    //
    // This can be allocated on the heap and should
    // have the correct alignment, allowing us to
    // use them as page tables with a physical
    // memory offset of 0.
    //
    #[derive(Clone)]
    #[repr(C)]
    #[repr(align(4096))]
    struct FakePageTable {
        entries: [u8; PhysFrameSize::Size4KiB.bytes()],
    }

    impl FakePageTable {
        fn new() -> Self {
            FakePageTable {
                entries: [0u8; PhysFrameSize::Size4KiB.bytes()],
            }
        }
    }

    // This is a "physical frame allocator" that
    // returns a virtual memory buffer that is not
    // otherwise in use. This means we can use it
    // to test page mapping with fake page tables
    // in userspace.
    //
    struct FakePhysFrameAllocator {
        buffers: Vec<Box<FakePageTable>>,
    }

    impl FakePhysFrameAllocator {
        fn new() -> Self {
            FakePhysFrameAllocator {
                buffers: Vec::new(),
            }
        }

        fn allocate(&mut self) -> PhysAddr {
            let next = Box::new(FakePageTable::new());
            let addr = PhysAddr::new(next.as_ref() as *const FakePageTable as usize);
            self.buffers.push(next);

            addr
        }
    }

    unsafe impl PhysFrameAllocator for FakePhysFrameAllocator {
        fn allocate_phys_frame(&mut self, size: PhysFrameSize) -> Option<PhysFrame> {
            if size != PhysFrameSize::Size4KiB {
                None
            } else {
                let addr = self.allocate();
                let frame = PhysFrame::from_start_address(addr, size)
                    .expect("got unaligned fake page table");
                Some(frame)
            }
        }
    }

    unsafe impl paging::FrameAllocator<paging::Size4KiB> for FakePhysFrameAllocator {
        fn allocate_frame(&mut self) -> Option<paging::PhysFrame> {
            let addr = self.allocate().as_x86_64();
            let frame =
                paging::PhysFrame::from_start_address(addr).expect("got unaligned fake page table");
            Some(frame)
        }
    }

    // To avoid a ridiculous amount of code duplication, particularly
    // in copying identical data between the local and x86_64 types,
    // we make heavy use of macros in the following code.
    //
    // The TestCase type below is used to store the data for a
    // single test case. Rather than having to duplicate the fields
    // of incompatible types (VirtPageSize and PhysFrameSize being
    // the obvious examples), we even use a macro to construct the
    // TestCase instances. The only notable factor here is that
    // we use a repeatable macro field for the page table flags so
    // we can avoid repeating the `PageTableFlags::` prefix and so
    // that we can use the same technique for the x86_64 page table
    // flags. It's a little unnecessary, but it makes the call site
    // significantly more concise, which makes it easier to compare
    // different test cases.
    //
    // We then use a mirror pair of macros for checking the translation
    // on a test case; one for this implementation, one for x86_64.
    // It's less necessary for these to be macros than the mapping
    // macro, but doing so means that any assertion failures will
    // report the position of the call site, which would not be the
    // case if we used a function instead.

    struct TestCase {
        name: &'static str,
        virt_addr: VirtAddr,
        virt_addr_x86_64: x86_64::VirtAddr,
        phys_addr: PhysAddr,
        phys_addr_x86_64: x86_64::PhysAddr,
        page: VirtPage,
        frame: PhysFrame,
        page_size: VirtPageSize,
        frame_size: PhysFrameSize,
        flags_in: PageTableFlags,
        flags_in_x86_64: paging::PageTableFlags,
        flags_out: PageTableFlags,
    }

    macro_rules! testcase {
        ($name:literal, $vaddr:expr, $paddr:expr, $size:ident, $($flags_in:ident)|+, $($flags_out:ident)|+,) => {
            TestCase {
                name: $name,
                virt_addr: VirtAddr::new($vaddr),
                virt_addr_x86_64: x86_64::VirtAddr::new($vaddr as u64),
                phys_addr: PhysAddr::new($paddr),
                phys_addr_x86_64: x86_64::PhysAddr::new($paddr as u64),
                page: VirtPage::from_start_address(VirtAddr::new($vaddr), VirtPageSize::$size)
                    .expect("VirtPage"),
                frame: PhysFrame::from_start_address(PhysAddr::new($paddr), PhysFrameSize::$size)
                    .expect("PhysFrame"),
                page_size: VirtPageSize::$size,
                frame_size: PhysFrameSize::$size,
                flags_in: $(PageTableFlags::$flags_in)|+,
                flags_in_x86_64: $(paging::PageTableFlags::$flags_in)|+,
                flags_out: $(PageTableFlags::$flags_out)|+,
            }
        };
    }

    // These are the mappings we want to create in the
    // test functions below.
    //
    fn test_cases() -> Vec<TestCase> {
        vec![
            testcase!(
                "4 KiB basic mapping",
                0x7123_4567_8000_usize,
                0x0007_edcb_a987_6000_usize,
                Size4KiB,
                PRESENT,
                PRESENT,
            ),
            testcase!(
                "4 KiB flags mapping",
                0x1000_usize,
                0x6000_usize,
                Size4KiB,
                PRESENT
                    | USER_ACCESSIBLE
                    | WRITE_THROUGH
                    | NO_CACHE
                    | WRITABLE
                    | GLOBAL
                    | NO_EXECUTE,
                PRESENT
                    | USER_ACCESSIBLE
                    | WRITE_THROUGH
                    | NO_CACHE
                    | WRITABLE
                    | GLOBAL
                    | NO_EXECUTE,
            ),
            testcase!(
                "2 MiB basic mapping",
                0x7234_5680_0000_usize,
                0x0007_1234_5660_0000_usize,
                Size2MiB,
                PRESENT,
                PRESENT | HUGE_PAGE,
            ),
            testcase!(
                "2 MiB flags mapping",
                0x7100_0000_0000_usize,
                0x007_1234_5660_0000_usize,
                Size2MiB,
                PRESENT
                    | USER_ACCESSIBLE
                    | WRITE_THROUGH
                    | NO_CACHE
                    | WRITABLE
                    | GLOBAL
                    | NO_EXECUTE,
                PRESENT
                    | USER_ACCESSIBLE
                    | WRITE_THROUGH
                    | NO_CACHE
                    | WRITABLE
                    | HUGE_PAGE
                    | GLOBAL
                    | NO_EXECUTE,
            ),
            testcase!(
                "1 GiB basic mapping",
                0xffff_8765_4000_0000_usize,
                0x0007_7654_0000_0000_usize,
                Size1GiB,
                PRESENT,
                PRESENT | HUGE_PAGE,
            ),
            testcase!(
                "1 GiB flags mapping",
                0xffff_8333_0000_0000_usize,
                0x0007_7654_0000_0000_usize,
                Size1GiB,
                PRESENT
                    | USER_ACCESSIBLE
                    | WRITE_THROUGH
                    | NO_CACHE
                    | WRITABLE
                    | GLOBAL
                    | NO_EXECUTE,
                PRESENT
                    | USER_ACCESSIBLE
                    | WRITE_THROUGH
                    | NO_CACHE
                    | WRITABLE
                    | HUGE_PAGE
                    | GLOBAL
                    | NO_EXECUTE,
            ),
        ]
    }

    macro_rules! check {
        ($page_table:expr, $mapping:expr) => {
            // Check that the page and frame match.
            assert_eq!(
                $page_table.translate_page($mapping.page),
                Some($mapping.frame),
                "{:?} translate_page",
                $mapping.name
            );

            // Check that the last address before the mapping
            // is not mapped.
            assert_eq!(
                $page_table.translate($mapping.virt_addr - 1),
                PageMapping::NotMapped,
                "{:?} translate last address before mapping",
                $mapping.name
            );

            // Check that the first address in the mapping
            // is mapped correctly.
            assert_eq!(
                $page_table.translate($mapping.virt_addr),
                PageMapping::Mapping {
                    frame: $mapping.frame,
                    addr: $mapping.phys_addr,
                    flags: $mapping.flags_out,
                },
                "{:?} translate first address in mapping",
                $mapping.name
            );

            // Check that the last address in the mapping
            // is mapped correctly.
            assert_eq!(
                $page_table.translate($mapping.virt_addr + $mapping.page_size.bytes() - 1),
                PageMapping::Mapping {
                    frame: $mapping.frame,
                    addr: $mapping.phys_addr + $mapping.frame_size.bytes() - 1,
                    flags: $mapping.flags_out,
                },
                "{:?} translate last address in mapping",
                $mapping.name
            );

            // Check that the first address after the
            // mapping is not mapped.
            assert_eq!(
                $page_table.translate($mapping.virt_addr + $mapping.page_size.bytes()),
                PageMapping::NotMapped,
                "{:?} translate first address after mapping",
                $mapping.name
            );
        };
    }

    macro_rules! check_x86_64 {
        ($page_table:expr, $mapping:expr) => {
            // x86_64::structures::paging::mapper::TranslateResult
            // doesn't implement PartialEq, so we can't use the
            // `translate` method with `assert_eq!`. Instead, we
            // fall back to the less precise `translate_addr`
            // method.

            // Check that the last address before the mapping
            // is not mapped.
            assert_eq!(
                $page_table.translate_addr($mapping.virt_addr_x86_64 - 1u64),
                None,
                "{:?} x86_64::translate_addr last address before mapping",
                $mapping.name
            );

            // Check that the first address in the mapping
            // is mapped correctly.
            assert_eq!(
                $page_table.translate_addr($mapping.virt_addr_x86_64),
                Some($mapping.phys_addr_x86_64),
                "{:?} x86_64::translate_addr first address in mapping",
                $mapping.name
            );

            // Check that the last address in the mapping
            // is mapped correctly.
            assert_eq!(
                $page_table.translate_addr(
                    $mapping.virt_addr_x86_64 + $mapping.page_size.bytes() as u64 - 1u64
                ),
                Some($mapping.phys_addr_x86_64 + $mapping.frame_size.bytes() as u64 - 1u64),
                "{:?} x86_64::translate_addr last address in mapping",
                $mapping.name
            );

            // Check that the first address after the
            // mapping is not mapped.
            assert_eq!(
                $page_table
                    .translate_addr($mapping.virt_addr_x86_64 + $mapping.page_size.bytes() as u64),
                None,
                "{:?} x86_64::translate_addr first address after mapping",
                $mapping.name
            );
        };
    }

    // Test that the translation process works
    // correctly by making page mappings using
    // the [`x86_64`] crate, then checking that
    // this implementation's translations agree.
    //
    #[test]
    fn test_page_table_translate() {
        // We pretend that we're using physical memory by using
        // an offset of 0.
        let offset = VirtAddr::zero();
        let cases = test_cases();

        // Make the level-4 page table.
        let mut allocator = FakePhysFrameAllocator::new();
        let mut pml4 = Box::new(paging::PageTable::new());
        let mut offset_page_table =
            unsafe { paging::OffsetPageTable::new(pml4.as_mut(), offset.as_x86_64()) };

        // Make our mappings.
        // This is a little clumsy, due to the
        // fact that the page and frame types
        // in x86_64 are generic, so we can't
        // store them in TestCase.

        for case in cases.iter() {
            match case.page_size {
                VirtPageSize::Size4KiB => unsafe {
                    offset_page_table
                        .map_to(
                            paging::Page::<paging::Size4KiB>::from_start_address(
                                case.virt_addr_x86_64,
                            )
                            .expect(case.name),
                            paging::PhysFrame::<paging::Size4KiB>::from_start_address(
                                case.phys_addr_x86_64,
                            )
                            .expect(case.name),
                            case.flags_in_x86_64,
                            &mut allocator,
                        )
                        .expect(case.name)
                        .ignore();
                },
                VirtPageSize::Size2MiB => unsafe {
                    offset_page_table
                        .map_to(
                            paging::Page::<paging::Size2MiB>::from_start_address(
                                case.virt_addr_x86_64,
                            )
                            .expect(case.name),
                            paging::PhysFrame::<paging::Size2MiB>::from_start_address(
                                case.phys_addr_x86_64,
                            )
                            .expect(case.name),
                            case.flags_in_x86_64,
                            &mut allocator,
                        )
                        .expect(case.name)
                        .ignore();
                },
                VirtPageSize::Size1GiB => unsafe {
                    offset_page_table
                        .map_to(
                            paging::Page::<paging::Size1GiB>::from_start_address(
                                case.virt_addr_x86_64,
                            )
                            .expect(case.name),
                            paging::PhysFrame::<paging::Size1GiB>::from_start_address(
                                case.phys_addr_x86_64,
                            )
                            .expect(case.name),
                            case.flags_in_x86_64,
                            &mut allocator,
                        )
                        .expect(case.name)
                        .ignore();
                },
            }
        }

        // Switch to our page table.
        drop(offset_page_table);
        let pml4_addr = PhysAddr::new(pml4.as_ref() as *const paging::PageTable as usize);
        let page_table = unsafe { PageTable::at_offset(pml4_addr, offset) };

        // Check the translations.
        for case in cases.iter() {
            check!(page_table, case);
        }
    }

    // Test that the page mapping process works
    // correctly by making page mappings using
    // the this implementation, then checking that
    // both the [`x86_64`] crate and this
    // implementation's translations agree.
    //
    #[test]
    fn test_page_table_map() {
        // We pretend that we're using physical memory by using
        // an offset of 0.
        let offset = VirtAddr::zero();
        let cases = test_cases();

        // Make the level-4 page table.
        let mut allocator = FakePhysFrameAllocator::new();
        let mut pml4 = Box::new(paging::PageTable::new());
        let pml4_addr = PhysAddr::new(pml4.as_ref() as *const paging::PageTable as usize);
        let mut page_table = unsafe { PageTable::at_offset(pml4_addr, offset) };

        // Make our mappings.

        for case in cases.iter() {
            unsafe {
                // Make the page mapping.
                page_table
                    .map(case.page, case.frame, case.flags_in, &mut allocator)
                    .expect(case.name)
                    .ignore();

                // Check that trying to make the
                // mapping again fails correctly.
                assert_eq!(
                    page_table
                        .map(case.page, case.frame, case.flags_in, &mut allocator)
                        .expect_err(case.name),
                    PageMappingError::PageAlreadyMapped(case.frame),
                    "{:?} second mapping",
                    case.name
                );
            }
        }

        // Check the translations with our page table.
        let pml4_addr = PhysAddr::new(pml4.as_ref() as *const paging::PageTable as usize);
        let page_table = unsafe { PageTable::at_offset(pml4_addr, offset) };

        // Check the translations.
        for case in cases.iter() {
            check!(page_table, case);
        }

        // Switch to the x86_64 page table.
        drop(page_table);
        let offset_page_table =
            unsafe { paging::OffsetPageTable::new(pml4.as_mut(), offset.as_x86_64()) };

        // Check the translations again.
        for case in cases.iter() {
            check_x86_64!(offset_page_table, case);
        }

        // Switch back to our page table.
        drop(offset_page_table);
        let mut page_table = unsafe { PageTable::at_offset(pml4_addr, offset) };

        // Unmap each mapping, checking that we
        // get the right physical address back
        // and that the mapping is removed.
        for case in cases.iter() {
            let (frame, flush) = unsafe { page_table.unmap(case.page) }.expect(case.name);
            flush.ignore();
            assert_eq!(frame, case.frame, "{:?} unmap frame", case.name);
            assert_eq!(
                page_table.translate_page(case.page),
                None,
                "{:?} translate page after unmap",
                case.name
            );
        }
    }
}
