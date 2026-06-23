.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0xff
    li a1, 0xf
    and a2, a0, a1
    li a3, 0xf
    sub a0, a2, a3
    li a7, 93
    ecall
