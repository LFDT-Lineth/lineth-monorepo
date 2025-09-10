(defclookup
  (mmio-into-kec :unchecked)
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
  mmio.EXO_IS_KEC
  ;; source columns
  (
    mmio.KEC_ID
    PHASE_KECCAK_DATA
    mmio.INDEX_X
    mmio.LIMB
    mmio.SIZE
    mmio.TOTAL_SIZE
  ))


