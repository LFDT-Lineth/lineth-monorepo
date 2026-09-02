//! Allocator backing: on the freestanding guest, a bump allocator over a static arena, reset at
//! the start of every C-ABI entry point (single hart, non-reentrant calls); hosted builds use the
//! platform allocator and both helpers are no-ops.

#[cfg(target_os = "none")]
mod imp {
    use core::alloc::{GlobalAlloc, Layout};
    use core::cell::UnsafeCell;

    const ARENA_BYTES: usize = 8 << 20;

    struct BumpArena {
        bytes: UnsafeCell<[u8; ARENA_BYTES]>,
        cursor: UnsafeCell<usize>,
    }

    unsafe impl Sync for BumpArena {}

    #[global_allocator]
    static ARENA: BumpArena = BumpArena {
        bytes: UnsafeCell::new([0; ARENA_BYTES]),
        cursor: UnsafeCell::new(0),
    };

    unsafe impl GlobalAlloc for BumpArena {
        unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
            let cursor = &mut *self.cursor.get();
            let start = match cursor.checked_add(layout.align() - 1) {
                Some(v) => v & !(layout.align() - 1),
                None => return core::ptr::null_mut(),
            };
            match start.checked_add(layout.size()) {
                Some(end) if end <= ARENA_BYTES => {
                    *cursor = end;
                    (self.bytes.get() as *mut u8).add(start)
                }
                _ => core::ptr::null_mut(),
            }
        }

        unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
            // Reclaims only a topmost allocation; interior frees are reclaimed wholesale by
            // `scope`/`reset`.
            let cursor = &mut *self.cursor.get();
            let offset = ptr as usize - self.bytes.get() as usize;
            if offset + layout.size() == *cursor {
                *cursor = offset;
            }
        }
    }

    pub fn reset() {
        unsafe { *ARENA.cursor.get() = 0 }
    }

    /// Runs `f` and releases every arena allocation it made. The `Copy` bound keeps heap-owning
    /// values from escaping the reclaimed region.
    pub fn scope<R: Copy>(f: impl FnOnce() -> R) -> R {
        let saved = unsafe { *ARENA.cursor.get() };
        let result = f();
        unsafe { *ARENA.cursor.get() = saved }
        result
    }
}

#[cfg(not(target_os = "none"))]
mod imp {
    pub fn reset() {}

    pub fn scope<R: Copy>(f: impl FnOnce() -> R) -> R {
        f()
    }
}

pub use imp::{reset, scope};
