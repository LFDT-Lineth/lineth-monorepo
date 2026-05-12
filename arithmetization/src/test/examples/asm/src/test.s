.section .rodata
value:
    .word 0x00000042

.section .text
.global _start
_start:
    # SP from linker script
    la      sp, _stack_start

    # Load the address of value into t0
    la      t0, value

    # Read the 32-bit word from memory into t1
    lw      t1, 0(t0)

    # Exit via ecall: a0 = exit code, a7 = 93 (syscall number for exit)
    li      a0, 0
    li      a7, 93
    ecall
    