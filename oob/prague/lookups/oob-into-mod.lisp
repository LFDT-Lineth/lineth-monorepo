(defun (oob-into-mod-activation-flag)
  oob.MOD_FLAG)

(defclookup
  oob-into-mod
  ;; target columns
  (
    mod.ARG_1_HI
    mod.ARG_1_LO
    mod.ARG_2_HI
    mod.ARG_2_LO
    mod.RES_HI
    mod.RES_LO
    mod.INST
  )
  ;; source selector
  (oob-into-mod-activation-flag)
  ;; source columns
  (
    [oob.OUTGOING_DATA 1]
    [oob.OUTGOING_DATA 2]
    [oob.OUTGOING_DATA 3]
    [oob.OUTGOING_DATA 4]
    0
    oob.OUTGOING_RES_LO
    oob.OUTGOING_INST
  ))


