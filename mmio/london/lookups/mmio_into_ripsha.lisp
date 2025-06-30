(deflookup
  mmio-into-ripsha
  ;reference columns
  (
    shakiradata.ID
    shakiradata.PHASE
    shakiradata.INDEX
    shakiradata.LIMB
    shakiradata.nBYTES
    shakiradata.TOTAL_SIZE
  )
  ;source columns
  (
    (* mmio.EXO_IS_RIPSHA mmio.EXO_ID)
    (* mmio.EXO_IS_RIPSHA mmio.PHASE)
    (* mmio.EXO_IS_RIPSHA mmio.INDEX_X)
    (* mmio.EXO_IS_RIPSHA mmio.LIMB)
    (* mmio.EXO_IS_RIPSHA mmio.SIZE)
    (* mmio.EXO_IS_RIPSHA mmio.TOTAL_SIZE)
  ))
