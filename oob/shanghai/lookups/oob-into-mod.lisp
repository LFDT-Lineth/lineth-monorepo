(defun (oob-into-mod-activation-flag)
  oob.MOD_FLAG)

(deflookup
  oob-into-mod
  ;source columns
  (
    mod.ARG_1_HI
    mod.ARG_1_LO
    mod.ARG_2_HI
    mod.ARG_2_LO
    mod.RES_HI
    mod.RES_LO
    mod.INST
  )
  ;target columns
  (
    (* [oob.OUTGOING_DATA 1] (oob-into-mod-activation-flag))
    (* [oob.OUTGOING_DATA 2] (oob-into-mod-activation-flag))
    (* [oob.OUTGOING_DATA 3] (oob-into-mod-activation-flag))
    (* [oob.OUTGOING_DATA 4] (oob-into-mod-activation-flag))
    (* 0 (oob-into-mod-activation-flag))
    (* oob.OUTGOING_RES_LO (oob-into-mod-activation-flag))
    (* oob.OUTGOING_INST (oob-into-mod-activation-flag))
  ))


