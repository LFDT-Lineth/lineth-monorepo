(deflookup
  mmio-into-kec
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
    (* mmio.EXO_IS_KEC mmio.KEC_ID)
    (* mmio.EXO_IS_KEC PHASE_KECCAK_DATA)
    (* mmio.EXO_IS_KEC mmio.INDEX_X)
    (* mmio.EXO_IS_KEC mmio.LIMB)
    (* mmio.EXO_IS_KEC mmio.SIZE)
    (* mmio.EXO_IS_KEC mmio.TOTAL_SIZE)
  ))


