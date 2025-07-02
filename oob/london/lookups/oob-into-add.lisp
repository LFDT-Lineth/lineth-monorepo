(defun (oob-into-add-activation-flag)
  oob.ADD_FLAG)

(defclookup
  oob-into-add
  ;; target columns
  (
    add.ARG_1_HI
    add.ARG_1_LO
    add.ARG_2_HI
    add.ARG_2_LO
    add.RES_HI
    add.RES_LO
    add.INST
  )
  ;; source selector
  (oob-into-add-activation-flag)
  ;; source columns
  (
    [oob.OUTGOING_DATA 1]
    [oob.OUTGOING_DATA 2]
    [oob.OUTGOING_DATA 3]
    [oob.OUTGOING_DATA 4]
    (next [oob.OUTGOING_DATA 1])
    (next [oob.OUTGOING_DATA 2])
    oob.OUTGOING_INST
  ))


