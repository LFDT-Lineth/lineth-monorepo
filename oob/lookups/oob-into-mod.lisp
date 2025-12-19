(defun (oob-into-mod-activation-flag)
  oob.MOD_FLAG)

(defclookup
  (oob-into-mod :unchecked)
  ;; target columns
  (
    mod.ARG_1
    mod.ARG_2
    mod.RES
    mod.INST
  )
  ;; source selector
  (oob-into-mod-activation-flag)
  ;; source columns
  (
    (:: [oob.OUTGOING_DATA 1] [oob.OUTGOING_DATA 2])
    (:: [oob.OUTGOING_DATA 3] [oob.OUTGOING_DATA 4])
    oob.OUTGOING_RES_LO
    oob.OUTGOING_INST
  ))


