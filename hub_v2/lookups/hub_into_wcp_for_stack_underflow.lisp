(defun (hub-into-wcp-for-sux-activation-flag) hub_v2.PEEK_AT_STACK)

(deflookup hub-into-wcp-for-sux
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
        (* EVM_INST_LT            (hub-into-wcp-for-sux-activation-flag))
        0
        (* hub_v2.HEIGHT          (hub-into-wcp-for-sux-activation-flag))
        0
        (* hub_v2.stack/DELTA     (hub-into-wcp-for-sux-activation-flag))
        (* hub_v2.stack/SUX       (hub-into-wcp-for-sux-activation-flag))
    )
)
