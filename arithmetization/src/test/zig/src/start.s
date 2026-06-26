.section .text
.global _start
_start:
    la sp, _stack_start  # SP from linker script
    call main