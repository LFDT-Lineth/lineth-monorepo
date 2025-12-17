(defun (oob-into-add-activation-flag)
  oob.ADD_FLAG)

(defclookup
  (oob-into-add :unchecked)
  ;; target columns
  (
    add.ARG_1
    add.ARG_2
    add.RES
    add.INST
    )
  ;; source selector
  (oob-into-add-activation-flag)
  ;; source columns
  (
    (:: [oob.OUTGOING_DATA 1] [oob.OUTGOING_DATA 2])
    (:: [oob.OUTGOING_DATA 3] [oob.OUTGOING_DATA 4])
    (:: (next [oob.OUTGOING_DATA 1]) (next [oob.OUTGOING_DATA 2]))
    oob.OUTGOING_INST
  ))


