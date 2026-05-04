.section .rodata
value:
    .word 0x8000000

.section .text
.global _start
_start:
    # Set SP explicitly (default is 0)
    li sp, 0x087fffff
    # sd      ra, -16(sp)

    # Load the address of value into t0
    la      t0, value

    # Read the 32-bit word from memory into t1
    lw      t1, 0(t0)

    # Exit via ecall: a0 = exit code, a7 = 93 (syscall number for exit)
    mv      a0, t1
    li      a7, 93
    ecall
    