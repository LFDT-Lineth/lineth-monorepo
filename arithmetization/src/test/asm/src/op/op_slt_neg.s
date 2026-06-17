.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0xffffffffffffffff
    li a1, 0x0
    slt a2, a0, a1
    li a3, 0x1
    sub a0, a2, a3
    li a7, 93
    ecall
