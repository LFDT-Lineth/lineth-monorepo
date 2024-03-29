(deflookup 
  mmio-into-logdata
  ;reference columns
  (
    logData.ABS_LOG_NUM
    logData.INDEX
    logData.LIMB
    logData.SIZE_LIMB
    logData.SIZE_TOTAL
  )
  ;source columns
  (
    (* mmio.EXO_IS_LOG mmio.EXO_ID)
    (* mmio.EXO_IS_LOG mmio.INDEX_X)
    (* mmio.EXO_IS_LOG mmio.LIMB)
    (* mmio.EXO_IS_LOG mmio.SIZE)
    (* mmio.EXO_IS_LOG mmio.TOTAL_SIZE)
  ))


