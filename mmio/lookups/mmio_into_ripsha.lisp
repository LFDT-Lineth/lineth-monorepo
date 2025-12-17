(defclookup
  (mmio-into-ripsha :unchecked)
  ;; target columns
  (
    shakiradata.ID
    shakiradata.PHASE
    shakiradata.INDEX
    shakiradata.LIMB
    shakiradata.nBYTES
    shakiradata.TOTAL_SIZE
  )
  ;; source selector
  mmio.EXO_IS_RIPSHA 
  ;; source columns
  (
    mmio.EXO_ID
    mmio.PHASE
    mmio.INDEX_X
    mmio.LIMB
    mmio.SIZE
    mmio.TOTAL_SIZE
  ))
