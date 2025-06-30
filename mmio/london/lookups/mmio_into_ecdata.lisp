(deflookup
  mmio-into-ecdata
  ;reference columns
  (
    ecdata.ID
    ecdata.PHASE
    ecdata.INDEX
    ecdata.LIMB
    ecdata.TOTAL_SIZE
    ecdata.SUCCESS_BIT
  )
  ;source columns
  (
    (* mmio.EXO_IS_ECDATA mmio.EXO_ID)
    (* mmio.EXO_IS_ECDATA mmio.PHASE)
    (* mmio.EXO_IS_ECDATA mmio.INDEX_X)
    (* mmio.EXO_IS_ECDATA mmio.LIMB)
    (* mmio.EXO_IS_ECDATA mmio.TOTAL_SIZE)
    (* mmio.EXO_IS_ECDATA mmio.SUCCESS_BIT)
  ))


