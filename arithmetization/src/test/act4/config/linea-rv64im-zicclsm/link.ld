/* References:                                                                          */
/* https://github.com/riscv/riscv-arch-test#linker-script                               */
/* https://github.com/riscv/riscv-arch-test/blob/act4/docs/memory_map.md#section-layout */
OUTPUT_ARCH( "riscv" )
ENTRY(rvtest_entry_point)

SECTIONS
{
  . = 0x00000000;
  .text.init    : { *(.text.init) }
  .text.rvtest  : { *(.text.rvtest) *(.text.rvtest.*) }
  . = ALIGN(0x4000);
  .data         : { *(.data) }
  . = ALIGN(0x1000);
  .text.rvmodel : { *(.text.rvmodel) *(.text.rvmodel.*) *(.text) *(.text.*) }
  . = ALIGN(0x1000);
  _end = .;
}
