(defclookup
  (mmio-into-ecdata :unchecked)
  ;; target columns
  (
    ecdata.ID
    ecdata.PHASE
    ecdata.INDEX
    ecdata.LIMB
    ecdata.TOTAL_SIZE
    ecdata.SUCCESS_BIT
  )
  ;; source selector
  mmio.EXO_IS_ECDATA
  ;; source columns
  (
   mmio.EXO_ID
   mmio.PHASE
   mmio.INDEX_X
   mmio.LIMB
   mmio.TOTAL_SIZE
   mmio.SUCCESS_BIT
  ))


