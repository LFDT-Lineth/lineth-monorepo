(deflookup
  mmio-into-blake2fmodexpdata
  ;reference columns
  (
    blake2fmodexpdata.ID
    blake2fmodexpdata.PHASE
    blake2fmodexpdata.INDEX
    blake2fmodexpdata.LIMB
  )
  ;source columns
  (
    (* mmio.EXO_IS_BLAKEMODEXP mmio.EXO_ID)
    (* mmio.EXO_IS_BLAKEMODEXP mmio.PHASE)
    (* mmio.EXO_IS_BLAKEMODEXP mmio.INDEX_X)
    (* mmio.EXO_IS_BLAKEMODEXP mmio.LIMB)
  ))


