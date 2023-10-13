(defun (non-zero-cfi)
  (if-not-zero rlpTxn.CODE_FRAGMENT_INDEX
               1))

(defplookup 
  rlpTxn-into-rom
  ;reference columns
  (
    rom.CODE_FRAGMENT_INDEX
    rom.LIMB
    rom.INDEX
    rom.nBYTES
  )
  ;source columns
  (
    rlpTxn.CODE_FRAGMENT_INDEX
    (* rlpTxn.LIMB (non-zero-cfi))
    (* rlpTxn.INDEX_DATA (non-zero-cfi))
    (* rlpTxn.nBYTES (non-zero-cfi))
  ))


