.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0xffffffffffffffff
    sltiu a2, a0, 0
    li a3, 0x0
    sub a0, a2, a3
    li a7, 93
    ecall
