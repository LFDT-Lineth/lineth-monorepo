const custom_std = @import("wrappers").custom_std;

// Linker-defined symbol whose address marks where runtime heap allocations can start
extern var _heap_start: u8;

export fn main() noreturn {
    // Read the heap start address
    const heap_start = @intFromPtr(&_heap_start);

    // A real allocator must fail if growing the heap would overflow the address space
    if (@addWithOverflow(heap_start, 1)[1] != 0) {
        custom_std.panic();
    }

    // Write to the first heap byte and read it back through a volatile pointer
    const allocation: *volatile u8 = @ptrFromInt(heap_start);
    allocation.* = 42;

    if (allocation.* != 42) {
        custom_std.panic();
    }

    custom_std.exit(0);
}
