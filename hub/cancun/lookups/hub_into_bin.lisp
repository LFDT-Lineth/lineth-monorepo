(defun (hub-into-bin-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/BIN_FLAG))

(deflookup hub-into-bin
    ;; target columns
    (
        bin.ARG_1_HI
        bin.ARG_1_LO
        bin.ARG_2_HI
        bin.ARG_2_LO
        bin.RES_HI
        bin.RES_LO
        bin.INST
    )
    ;; source columns
    (
        (* [hub.stack/STACK_ITEM_VALUE_HI 1]     (hub-into-bin-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 1]     (hub-into-bin-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 2]     (hub-into-bin-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 2]     (hub-into-bin-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 4]     (hub-into-bin-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 4]     (hub-into-bin-activation-flag))
        (*  hub.stack/INSTRUCTION                (hub-into-bin-activation-flag))
    )
)
