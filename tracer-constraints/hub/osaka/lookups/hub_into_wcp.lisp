(defun (hub-into-wcp-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/WCP_FLAG))

(defclookup
    (hub-into-wcp :unchecked)
    ;; target columns
    (
        wcp.ARG_1
        wcp.ARG_2
        wcp.RES
        wcp.INST
    )
    ;; source selector
    (hub-into-wcp-activation-flag)
    ;; source columns
    (
     (:: [hub.stack/STACK_ITEM_VALUE_HI 1] [hub.stack/STACK_ITEM_VALUE_LO 1]) ;; arg1
     (:: [hub.stack/STACK_ITEM_VALUE_HI 2] [hub.stack/STACK_ITEM_VALUE_LO 2]) ;; arg2
     [hub.stack/STACK_ITEM_VALUE_LO 4] ;; result
     hub.stack/INSTRUCTION
   ))
