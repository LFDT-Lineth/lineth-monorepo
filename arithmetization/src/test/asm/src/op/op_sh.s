.section .data
.balign 8
buf:
    .dword 0

.section .text
.global _start
_start:
    la sp, _stack_start
    la t0, buf
    li t1, 0xabcd
    sh t1, 0(t0)
    lhu a2, 0(t0)
    li a3, 0xabcd
    sub a0, a2, a3
    li a7, 93
    ecall
