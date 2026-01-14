(defun (hub-into-wcp-for-sox-activation-flag)
  (* hub.PEEK_AT_STACK (- 1 hub.stack/SUX)))

(defun ((projected-height :i16 :force))
  (* (- (+ hub.HEIGHT hub.stack/ALPHA) hub.stack/DELTA)
     (- 1 hub.stack/SUX)))

(defclookup
  hub-into-wcp-for-sox
  ;; target columns
  (
    wcp.INST
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
  )
  ;; source selector
  (hub-into-wcp-for-sox-activation-flag)
  ;; source columns
  (
    EVM_INST_GT
    (projected-height)
    1024
    hub.stack/SOX
 ))
