(defclookup
  (mmio-into-rlptxn :unchecked)
  ;; target columns
  (
    rlptxn.ABS_TX_NUM
    rlptxn.LC
    rlptxn.PHASE
    rlptxn.INDEX_DATA
    rlptxn.LIMB
  )
  ;; source selector
  mmio.EXO_IS_TXCD
  ;; source columns
  (
    mmio.EXO_ID
    1
    mmio.PHASE
    mmio.INDEX_X
    mmio.LIMB
  ))


