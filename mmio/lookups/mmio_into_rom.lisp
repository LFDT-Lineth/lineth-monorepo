(deflookup 
  mmio-into-rom
  ;reference columns
  (
    rom.CFI
    rom.INDEX
    rom.LIMB
    rom.nBYTES
    rom.CODE_SIZE
  )
  ;source columns
  (
    (* mmio.EXO_IS_ROM mmio.EXO_ID)
    (* mmio.EXO_IS_ROM mmio.INDEX_X)
    (* mmio.EXO_IS_ROM mmio.LIMB)
    (* mmio.EXO_IS_ROM mmio.SIZE)
    (* mmio.EXO_IS_ROM mmio.TOTAL_SIZE)
  ))


