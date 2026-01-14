(defun (sel-rlptxn-to-rom) (* rlptxn.IS_DEPLOYMENT rlptxn.REQUIRES_EVM_EXECUTION (is-limb-content-analysis-row)))

(defun (is-limb-content-analysis-row)           (* (prev rlptxn.CMP) rlptxn.CMP rlptxn.IS_DATA))

(defclookup
  rlptxn-into-rom
  ;; target columns
  (
    rom.CODE_FRAGMENT_INDEX
    rom.LIMB
    rom.INDEX
    rom.nBYTES
  )
  ;; source selector
  (sel-rlptxn-to-rom)
  ;; source columns
  (
    rlptxn.CODE_FRAGMENT_INDEX
    rlptxn.cmp/LIMB
    rlptxn.CT
    rlptxn.cmp/LIMB_SIZE
  ))


