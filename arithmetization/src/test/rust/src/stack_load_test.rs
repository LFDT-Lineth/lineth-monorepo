#![no_std]
#![no_main]

// Regression test for the stack-alignment bug we hit while debugging blake.
//
// Allocates a `[u64; 16]` on the stack, writes distinct values to each slot,
// then reads each slot back through a *dynamic* (loop-variable) index.
// `black_box` defeats const-folding so the loads/stores actually run.
//
// Before the linker-script fix (`_stack_start` was odd), some byte of one
// of these reads would come back zero. With the fix, every read matches.
//
// Exit codes:
//   0                  → all 16 reads matched expected values (PASS)
//   (i+1)*1000 + low12 → element i diverged; low 12 bits of the wrong value
//                        are encoded so you can tell whether it's "all
//                        zeros", a partial load, or something else.

include!("custom_std.rs");

#[no_mangle]
fn main() -> ! {
    let mut arr: [u64; 16] = [0u64; 16];

    let mut i: usize = 0;
    while i < 16 {
        arr[i] = 0x1122334455667780u64 + (i as u64);
        i += 1;
    }

    let mut j: usize = 0;
    while j < 16 {
        let expected = 0x1122334455667780u64 + (j as u64);
        let actual = core::hint::black_box(arr[j]);
        if actual != expected {
            exit(((j + 1) as i32) * 1000 + ((actual & 0xfff) as i32));
        }
        j += 1;
    }

    exit(0);
}
