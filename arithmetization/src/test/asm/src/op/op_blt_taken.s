.section .text
.global _start
_start:
    la sp, _stack_start
    li a0, 0xffffffffffffffff
    li a1, 0x0
    blt a0, a1, taken
    li a0, 1
    j done
taken:
    li a0, 0
done:
    li a7, 93
    ecall
