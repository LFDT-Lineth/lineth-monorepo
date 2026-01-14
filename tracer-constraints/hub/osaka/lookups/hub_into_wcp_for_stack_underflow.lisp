(defun (hub-into-wcp-for-sux-activation-flag) hub.PEEK_AT_STACK)

(defclookup hub-into-wcp-for-sux
    ;; target columns
    (
        wcp.INST
        wcp.ARG_1
        wcp.ARG_2
        wcp.RES
    )
    ;; source selector
    (hub-into-wcp-for-sux-activation-flag)
    ;; source columns
    (
        EVM_INST_LT
        hub.HEIGHT
        hub.stack/DELTA
        hub.stack/SUX
    )
)
