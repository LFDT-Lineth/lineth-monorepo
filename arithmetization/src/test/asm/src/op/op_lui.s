.section .text
.global _start
_start:
    la sp, _stack_start
    lui a2, 1
    li a3, 0x1000
    sub a0, a2, a3
    li a7, 93
    ecall
