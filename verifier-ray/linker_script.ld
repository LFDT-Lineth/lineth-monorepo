ENTRY(_start)

MEMORY {
    PROGRAM (rx) : ORIGIN = 0x00000000, LENGTH = 0x8000000
    STACK   (rw) : ORIGIN = 0x08000000, LENGTH = 0x800000
    BACKING (rw) : ORIGIN = 0x08800000, LENGTH = 0x10000000
    IN      (r)  : ORIGIN = 0x18800000, LENGTH = 0x30000000
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

    /* decoded proof backing storage */
    _proof_backing_start = ORIGIN(BACKING);
    _proof_backing_end = ORIGIN(BACKING) + LENGTH(BACKING);

    /* symbol marking the start of the input region */
    _in_start = ORIGIN(IN);
}
