.section .rodata
.balign 8
data:
    .byte 0x42
    .byte 0x55
    .byte 0x66
    .byte 0x77
    .byte 0x88
    .byte 0x99
    .byte 0xaa
    .byte 0xbb

.section .text
.global _start
_start:
    la sp, _stack_start
    la t0, data
    lhu a2, 4(t0)
    li a3, 0x9988
    sub a0, a2, a3
    li a7, 93
    ecall
