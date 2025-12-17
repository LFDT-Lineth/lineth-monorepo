(defclookup
  (mmio-into-blsdata :unchecked)
  ;; target columns
  (
    blsdata.ID
    blsdata.PHASE
    blsdata.INDEX
    blsdata.LIMB
    blsdata.TOTAL_SIZE
    blsdata.SUCCESS_BIT
  )
  ;; source selector
  mmio.EXO_IS_BLS
  ;; source columns
  (
   mmio.EXO_ID
   mmio.PHASE
   mmio.INDEX_X
   mmio.LIMB
   mmio.TOTAL_SIZE
   mmio.SUCCESS_BIT
  ))


