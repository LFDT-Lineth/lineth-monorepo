# `_start` Entry Point

## Question

Can I fix the address that the `_start` symbol points to, and impose that it is the first byte of the `.text` section?

## Answer

The linker script controls where `.text` lands (e.g. `0x80000000` in ZisK), and `_start`'s address is set to whatever the ELF entry point field says — which lands somewhere in ROM.

### Can you fix `_start` to a known address?

**Yes.** The linker script controls the origin of `.text`, so `_start` gets a fixed address (e.g. `0x80000000`) as long as it is placed first in the section.

### Can you guarantee `_start` is the first byte of `.text`?

This requires two things:

a. The linker script must emit `.text` starting at the desired address (e.g. `ORIGIN = 0x80000000`) — trivially done.

b. `_start` must be the very first symbol emitted into `.text`. This is **not automatic** — the linker concatenates `.text` sections from all `.o` files in link order, and `_start` ends up first only if its `.o` is linked first (or you explicitly force it).

### Reliable approach

Use the linker script's `ENTRY(_start)` directive (sets the ELF entry point field) and explicitly place `_start`'s section first:

```ld
SECTIONS {
    .text : {
        *(.text._start)   /* force _start section first */
        *(.text*)
    } > ROM
}
```

This requires compiling with `-ffunction-sections` so `_start` gets its own `.text._start` subsection, then naming it explicitly in the script. Without that, you're relying on link order, which is fragile.
