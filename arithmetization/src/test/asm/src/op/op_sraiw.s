.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0xfffffff0
    sraiw a2, a0, 1
    li a3, 0xfffffffffffffff8
    sub a0, a2, a3
    li a7, 93
    ecall
