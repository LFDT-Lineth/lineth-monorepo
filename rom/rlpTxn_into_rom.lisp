(defun (selector)
  (* (~ rlpTxn.CODE_FRAGMENT_INDEX) [rlpTxn.PHASE 9] (- 1 rlpTxn.IS_PREFIX) rlpTxn.LC))

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
    (* rlpTxn.CODE_FRAGMENT_INDEX (selector))
    (* rlpTxn.LIMB (selector))
    (* rlpTxn.INDEX_DATA (selector))
    (* rlpTxn.nBYTES (selector))
  ))


