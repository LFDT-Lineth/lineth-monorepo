    .section .rodata
value:
    .word 42

    .section .text
    .global _start
_start:
    # Load the address of value into t0
    la      t0, value

    # Read the 32-bit word from memory into t1
    lw      t1, 0(t0)

    # Exit via ecall: a7 = 93 (exit), a0 = exit code
    mv      a0, t1
    li      a7, 93
    ecall
