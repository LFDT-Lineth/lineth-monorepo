.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0x5
    addi a2, a0, 7
    li a3, 0xc
    sub a0, a2, a3
    li a7, 93
    ecall
