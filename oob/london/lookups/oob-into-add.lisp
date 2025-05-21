(defun (mxp-into-add-activation-flag)
  oob.ADD_FLAG)

(deflookup
  oob-into-add
  ;source columns
  (
    add.ARG_1_HI
    add.ARG_1_LO
    add.ARG_2_HI
    add.ARG_2_LO
    add.RES_HI
    add.RES_LO
    add.INST
  )
  ;target columns
  (
    (* [oob.OUTGOING_DATA 1] (mxp-into-add-activation-flag))
    (* [oob.OUTGOING_DATA 2] (mxp-into-add-activation-flag))
    (* [oob.OUTGOING_DATA 3] (mxp-into-add-activation-flag))
    (* [oob.OUTGOING_DATA 4] (mxp-into-add-activation-flag))
    (* (next [oob.OUTGOING_DATA 1]) (mxp-into-add-activation-flag))
    (* (next [oob.OUTGOING_DATA 2]) (mxp-into-add-activation-flag))
    (* oob.OUTGOING_INST (mxp-into-add-activation-flag))
  ))


