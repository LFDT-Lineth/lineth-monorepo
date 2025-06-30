(deflookup
  mmio-into-logdata
  ;reference columns
  (
    logdata.ABS_LOG_NUM
    logdata.INDEX
    logdata.LIMB
    logdata.SIZE_LIMB
    logdata.SIZE_TOTAL
  )
  ;source columns
  (
    (* mmio.EXO_IS_LOG mmio.EXO_ID)
    (* mmio.EXO_IS_LOG mmio.INDEX_X)
    (* mmio.EXO_IS_LOG mmio.LIMB)
    (* mmio.EXO_IS_LOG mmio.SIZE)
    (* mmio.EXO_IS_LOG mmio.TOTAL_SIZE)
  ))


