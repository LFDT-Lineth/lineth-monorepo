(defun (hub-into-wcp-for-sox-activation-flag)
  (* hub_v2.PEEK_AT_STACK
     (- 1 hub_v2.stack/SUX)))

(defun (projected-height)
  (- (+ hub_v2.HEIGHT
        hub_v2.stack/ALPHA)
     hub_v2.stack/DELTA))

(deflookup hub-into-wcp-for-sox
    ;; target columns
    (
        wcp.INST
        wcp.ARG_1_HI
        wcp.ARG_1_LO
        wcp.ARG_2_HI
        wcp.ARG_2_LO
        wcp.RESULT
    )
    ;; source columns
    (
        (* EVM_INST_GT           (hub-into-wcp-for-sox-activation-flag))
        0
        (* (projected-height)    (hub-into-wcp-for-sox-activation-flag))
        0
        (* 1024                  (hub-into-wcp-for-sox-activation-flag))
        (* hub_v2.stack/SOX      (hub-into-wcp-for-sox-activation-flag))
    )
)
