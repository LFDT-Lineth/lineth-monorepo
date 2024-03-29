(deflookup 
  mmio-into-blakemodexp
  ;reference columns
  (
    blake2f_modexp_data.ID
    blake2f_modexp_data.PHASE
    blake2f_modexp_data.INDEX
    blake2f_modexp_data.LIMB
  )
  ;source columns
  (
    (* mmio.EXO_IS_BLAKEMODEXP mmio.EXO_ID)
    (* mmio.EXO_IS_BLAKEMODEXP mmio.PHASE)
    (* mmio.EXO_IS_BLAKEMODEXP mmio.INDEX_X)
    (* mmio.EXO_IS_BLAKEMODEXP mmio.LIMB)
  ))


