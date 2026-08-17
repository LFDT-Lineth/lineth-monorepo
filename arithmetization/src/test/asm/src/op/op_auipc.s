.section .text
.global _start
_start:
    la sp, _stack_start
    auipc a0, 0
    auipc a1, 0
    sub a2, a1, a0
    addi a0, a2, -4
    li a7, 93
    ecall
