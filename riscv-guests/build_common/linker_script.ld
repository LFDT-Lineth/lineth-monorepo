ENTRY(_start)

MEMORY {
    STACK  (rw) : ORIGIN = 0x00000000, LENGTH = 0x00800000
    PROGRAM (rx) : ORIGIN = 0x00800000, LENGTH = 0x08000000
    IN      (r)  : ORIGIN = 0x08800000, LENGTH = 0x40000000
    /* HEAP  (rw) : ORIGIN = 0x48800000, LENGTH = infinite */
}

SECTIONS {
    . = ORIGIN(PROGRAM);
    _program_start = ORIGIN(PROGRAM);

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

    /* stack grows downward from the top of the stack region */
    _stack_end = ORIGIN(STACK);
    _stack_start = ORIGIN(STACK) + LENGTH(STACK);

    /* program and input regions grow upward */
    _program_end = ORIGIN(PROGRAM) + LENGTH(PROGRAM);
    _in_start = ORIGIN(IN);
    _in_end = ORIGIN(IN) + LENGTH(IN);
    /* aliases */
    _input_start = _in_start;
    _input_end = _in_end;

    /* heap grows upward from this address by program convention */
    _heap_start = 0x48800000;
}
