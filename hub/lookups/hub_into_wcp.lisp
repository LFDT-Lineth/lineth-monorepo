(defun (hub-into-wcp-activation-flag)
  (and (unexceptional-stack-row)
       hub.stack/WCP_FLAG))

(deflookup hub-into-wcp
    ;; target columns
    (
        wcp.ARG_1_HI
        wcp.ARG_1_LO
        wcp.ARG_2_HI
        wcp.ARG_2_LO
        wcp.RESULT
        wcp.INST
    )
    ;; source columns
    (
        (* [hub.stack/STACK_ITEM_VALUE_HI 1]     (hub-into-wcp-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 1]     (hub-into-wcp-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 2]     (hub-into-wcp-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 2]     (hub-into-wcp-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 4]     (hub-into-wcp-activation-flag))
        (* hub.stack/INSTRUCTION                 (hub-into-wcp-activation-flag))
    )
)
