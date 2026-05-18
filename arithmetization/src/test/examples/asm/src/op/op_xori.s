.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0xff
    xori a2, a0, 15
    li a3, 0xf0
    sub a0, a2, a3
    li a7, 93
    ecall
