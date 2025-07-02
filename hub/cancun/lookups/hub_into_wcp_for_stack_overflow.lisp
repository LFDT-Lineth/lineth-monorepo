(defun (hub-into-wcp-for-sox-activation-flag)
  (* hub.PEEK_AT_STACK (- 1 hub.stack/SUX)))

(defun (projected-height)
  (* (- (+ hub.HEIGHT hub.stack/ALPHA) hub.stack/DELTA)
     (- 1 hub.stack/SUX)))

(defclookup
  hub-into-wcp-for-sox
  ;; target columns
  (
    wcp.INST
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RESULT
  )
  ;; source selector
  (hub-into-wcp-for-sox-activation-flag)
  ;; source columns
  (
    EVM_INST_GT
    0
    (projected-height)
    0
    1024
    hub.stack/SOX
 ))
