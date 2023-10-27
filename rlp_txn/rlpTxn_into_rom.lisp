;; The source columns are the LIMB, when the CFI is not 0, in PHASE 9 of the Rlp module (data phase), not in its prefix phase, and when the LIMB is constructed (LC=1)
(defun (sel-rlptxn-to-rom)
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
    (* rlpTxn.CODE_FRAGMENT_INDEX (sel-rlptxn-to-rom))
    (* rlpTxn.LIMB (sel-rlptxn-to-rom))
    (* rlpTxn.INDEX_DATA (sel-rlptxn-to-rom))
    (* rlpTxn.nBYTES (sel-rlptxn-to-rom))
  ))


