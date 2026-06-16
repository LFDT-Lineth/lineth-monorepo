# Note:
# This program sums 1024 and 2048 and checks if IN_BYTES contains the exepcted result 3072.
# 
# v0 = 0x0000000000000400
# v1 = 0x0000000000000800
# v3 = IN_BYTES = 0x0000000000000c00 (big-endian), written as 0x000c000000000000 in RAM
# v0 + v1 ?= v2
#
# To run:
# riscv-test test.s IN_BYTES="0x0000000000000c00" (pass)
# riscv-test test.s IN_BYTES="0x0000000000000042" (fail)
.section .data
v0:
    .dword 0x0000000000000400

.section .rodata
v1:
    .dword 0x0000000000000800 

.section .text
.global _start
_start:
    # SP from linker script
    la      sp, _stack_start
    
    # Load address of vx into tx and then load its value into tx
    la      t0, v0
    ld      t0, 0(t0)
    
    la      t1, v1
    ld      t1, 0(t1)

    la      t2, _in_start
    ld      t2, 0(t2)

    # Sum v0 and v1 
    add     t3, t0, t1     
    
    # Exit via ecall: a0 = exit code, a7 = 93 (syscall number for exit)
    # exic code is 0 if IN_BYTES is equal to the computed result
    xor     a0, t2, t3
    li      a7, 93
    ecall
    
