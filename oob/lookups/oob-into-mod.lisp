(defun (mod-activation-flag)
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
    (* [oob.OUTGOING_DATA 1] (mod-activation-flag))
    (* [oob.OUTGOING_DATA 2] (mod-activation-flag))
    (* [oob.OUTGOING_DATA 3] (mod-activation-flag))
    (* [oob.OUTGOING_DATA 4] (mod-activation-flag))
    (* 0 (mod-activation-flag))
    (* oob.OUTGOING_RES_LO (mod-activation-flag))
    (* oob.OUTGOING_INST (mod-activation-flag))
  ))


