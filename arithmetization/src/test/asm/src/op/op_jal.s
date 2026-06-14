.section .text
.global _start
_start:
    la sp, _stack_start
    jal x0, target
    li a0, 1
    j done
target:
    li a0, 0
done:
    li a7, 93
    ecall
