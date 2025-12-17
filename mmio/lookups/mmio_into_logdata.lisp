(defclookup
  (mmio-into-logdata :unchecked)
  ;; target columns
  (
    logdata.ABS_LOG_NUM
    logdata.INDEX
    logdata.LIMB
    logdata.SIZE_LIMB
    logdata.SIZE_TOTAL
  )
  ;; source selector
  mmio.EXO_IS_LOG 
  ;; source columns
  (
    mmio.EXO_ID
    mmio.INDEX_X
    mmio.LIMB
    mmio.SIZE
    mmio.TOTAL_SIZE
  ))


