# start.s
.section .text
.global _start
_start:
    li sp, 0x087fffff  # set stack pointer to a known memory region
    call main