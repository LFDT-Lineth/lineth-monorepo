;; The source columns are the LIMB, when the CFI is not 0, in PHASE 9 of the Rlp module (data phase), not in its prefix phase, and when the LIMB is constructed (LC=1)
(defun (sel-rlptxn-to-rom)
  (* (~ rlptxn.CODE_FRAGMENT_INDEX) rlptxn.IS_PHASE_DATA (- 1 rlptxn.IS_PREFIX) rlptxn.LC))

(deflookup
  rlptxn-into-rom
  ;; target columns
  (
    rom.CODE_FRAGMENT_INDEX
    rom.LIMB
    rom.INDEX
    rom.nBYTES
  )
  ;; source columns
  (
    (* rlptxn.CODE_FRAGMENT_INDEX (sel-rlptxn-to-rom))
    (* rlptxn.LIMB (sel-rlptxn-to-rom))
    (* rlptxn.INDEX_DATA (sel-rlptxn-to-rom))
    (* rlptxn.nBYTES (sel-rlptxn-to-rom))
  ))


