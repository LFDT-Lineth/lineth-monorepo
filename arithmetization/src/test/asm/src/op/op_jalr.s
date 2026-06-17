.section .text
.global _start
_start:
    la sp, _stack_start
    la t0, target
    jalr x0, 0(t0)
    li a0, 1
    j done
target:
    li a0, 0
done:
    li a7, 93
    ecall
