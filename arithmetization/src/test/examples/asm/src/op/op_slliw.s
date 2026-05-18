.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0x1
    slliw a2, a0, 4
    li a3, 0x10
    sub a0, a2, a3
    li a7, 93
    ecall
