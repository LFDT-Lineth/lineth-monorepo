// rvmodel_macros.h for Linea (RV64IM_Zicclsm ZK-VM target)
// PASS: a0=0, a7=93 (success exit code)
// FAIL: a0=1, ay=93 (non-zero exit code)
// SPDX-License-Identifier: BSD-3-Clause

#ifndef _COMPLIANCE_MODEL_H
#define _COMPLIANCE_MODEL_H

#define RVMODEL_DATA_SECTION \
        .pushsection .tohost,"aw",@progbits;                \
        .align 8; .global tohost; tohost: .dword 0;         \
        .align 8; .global fromhost; fromhost: .dword 0;     \
        .popsection

#define RVMODEL_BOOT

#define RVMODEL_HALT_PASS  \
  li a0, 0                ;\
  li a7, 93               ;\
  ecall                   ;\
  j .                     ;\

#define RVMODEL_HALT_FAIL \
  li a0, 1                ;\
  li a7, 93               ;\
  ecall                   ;\
  j .                     ;\

#define RVMODEL_IO_INIT(_R1, _R2, _R3)

// Print a NUL-terminated string by issuing the Linux RISC-V `write` syscall
// (a7=64, a0=fd, a1=buf, a2=count). Our zkc interpreter (see
// `process_syscall` in `instruction_processing/i_type.zkc`) recognises
// syscall 64 and dumps the buffer as space-separated %x bytes on the
// printf channel — zkc has no `%c` format specifier, so failure messages
// surface as hex codes (decode with e.g. `printf '\\x%s' XX XX ...`).
//
// Per the framework contract (cf. tests/env/sail_macros.h), _STR_PTR is a
// register that already holds the pointer to the string. _R1/_R2/_R3 are
// scratch. Live a0/a1/a2/a7 are saved/restored around the ecall so the
// macro is safe to invoke mid-test without clobbering the caller's a-regs.
#define RVMODEL_IO_WRITE_STR(_R1, _R2, _R3, _STR_PTR)                       \
        mv       _R1, _STR_PTR                                             ;\
        mv       _R2, _STR_PTR                                             ;\
    1:  lbu      _R3, 0(_R2)                                               ;\
        beqz     _R3, 2f                                                   ;\
        addi     _R2, _R2, 1                                               ;\
        j        1b                                                        ;\
    2:  sub      _R2, _R2, _R1                                             ;\
        addi     sp,  sp,  -32                                             ;\
        sd       a0,  0(sp)                                                ;\
        sd       a1,  8(sp)                                                ;\
        sd       a2,  16(sp)                                               ;\
        sd       a7,  24(sp)                                               ;\
        li       a0,  1                                                    ;\
        mv       a1,  _R1                                                  ;\
        mv       a2,  _R2                                                  ;\
        li       a7,  64                                                   ;\
        ecall                                                              ;\
        ld       a0,  0(sp)                                                ;\
        ld       a1,  8(sp)                                                ;\
        ld       a2,  16(sp)                                               ;\
        ld       a7,  24(sp)                                               ;\
        addi     sp,  sp,  32

#define RVMODEL_ACCESS_FAULT_ADDRESS 0x00000000

#define RVMODEL_MTIME_ADDRESS    0x02004000

#define RVMODEL_MTIMECMP_ADDRESS 0x02000000

// Linea does not implement timer interrupts, but the framework's
// check_defines.h still requires the macro to be defined.  Use a
// placeholder value (timer never fires soon).
#define RVMODEL_TIMER_INT_SOON_DELAY 0

// Linea does not implement external/software interrupts, but the
// framework's check_defines.h still requires the macro to be defined.
#define RVMODEL_INTERRUPT_LATENCY 0

#define RVMODEL_SET_MEXT_INT

#define RVMODEL_CLR_MEXT_INT

#define RVMODEL_SET_MSW_INT

#define RVMODEL_CLR_MSW_INT

#define RVMODEL_SET_SEXT_INT

#define RVMODEL_CLR_SEXT_INT

#define RVMODEL_SET_SSW_INT

#define RVMODEL_CLR_SSW_INT

#endif // _COMPLIANCE_MODEL_H
