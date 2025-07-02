(defun (hub-into-wcp-for-sux-activation-flag) hub.PEEK_AT_STACK)

(defclookup hub-into-wcp-for-sux
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
    (hub-into-wcp-for-sux-activation-flag)
    ;; source columns
    (
        EVM_INST_LT
        0
        hub.HEIGHT
        0
        hub.stack/DELTA
        hub.stack/SUX
    )
)
