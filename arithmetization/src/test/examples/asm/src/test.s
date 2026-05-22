.section .rodata
value:
    .word 0x00000042
.section .data
another_value:
    .word 0x00000069

.section .text
.global _start
_start:
    # SP from linker script
    la      sp, _stack_start

    # Load the address of value into t0
    la      t0, value

    # Load the address of another_value into t1
    la      t1, another_value

    # Read 32-bit word from memory into t2 using t0 as offset
    lw      t2, 0(t0)

    # Read 32-bit word from memory into t3 using t1 as offset
    lw      t3, 0(t1)

    # Exit via ecall: a0 = exit code, a7 = 93 (syscall number for exit)
    li      a0, 0
    li      a7, 93
    ecall
    