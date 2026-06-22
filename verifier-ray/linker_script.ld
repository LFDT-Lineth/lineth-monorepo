ENTRY(_start)

MEMORY {
    PROGRAM (rx) : ORIGIN = 0x00000000, LENGTH = 0x8000000
    STACK   (rw) : ORIGIN = 0x08000000, LENGTH = 0x800000
    IN  (r)  : ORIGIN = 0x08800000, LENGTH = 0x40000000
}

SECTIONS {
    . = ORIGIN(PROGRAM);

    .text : {
        *(.text .text.*)
    } > PROGRAM

    .rodata : {
        *(.rodata .rodata.*)
    } > PROGRAM

    .data : {
        *(.data .data.*)
    } > PROGRAM

    .bss : {
        *(.bss .bss.*)
    } > PROGRAM

    /DISCARD/ : {
        *(.eh_frame .eh_frame.*)
        *(.eh_frame_hdr)
    }

    /* stack grows downward from the top of the stack region */
    _stack_start = ORIGIN(STACK) + LENGTH(STACK);

    /* symbol marking the start of the input region */
    _in_start = ORIGIN(IN);
}
