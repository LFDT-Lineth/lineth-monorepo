.section .data
.balign 8
buf:
    .dword 0

.section .text
.global _start
_start:
    la sp, _stack_start
    la t0, buf
    li t1, 0x12345678
    sw t1, 0(t0)
    lwu a2, 0(t0)
    li a3, 0x12345678
    sub a0, a2, a3
    li a7, 93
    ecall
