(defun (hub-into-wcp-activation-flag) 
  (and (unexceptional-stack-row)
       hub_v2.stack/WCP_FLAG))

(deflookup hub-into-wcp
    ;; target columns
    (
        wcp.ARG_1_HI
        wcp.ARG_1_LO
        wcp.ARG_2_HI
        wcp.ARG_2_LO
        ;; 0
        wcp.RESULT
        wcp.INST
    )
    ;; source columns
    (
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1]     (hub-into-wcp-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1]     (hub-into-wcp-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 3]     (hub-into-wcp-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 3]     (hub-into-wcp-activation-flag))
        ;; (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4]     (hub-into-wcp-activation-flag)) ;; TODO: cheaper alternative to setting the HI part to 0 in the HUB
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4]     (hub-into-wcp-activation-flag))
        (* hub_v2.stack/INSTRUCTION                 (hub-into-wcp-activation-flag))
    )
)
