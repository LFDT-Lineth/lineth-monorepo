(defclookup
  (mmio-into-rlptxn :unchecked)
  ;; target columns
  (
    rlptxn.USER_TXN_NUMBER
    (is-limb-content-analysis-row)
    rlptxn.CT
    rlptxn.cmp/LIMB
  )
  ;; source columns
  mmio.EXO_IS_TXCD
  ;; source columns
  (
    mmio.EXO_ID
    1
    mmio.INDEX_X
    mmio.LIMB
  ))


