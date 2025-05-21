(defun (hub-into-ext-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/EXT_FLAG))

(deflookup hub-into-ext
    ;; target columns
    (
        ext.ARG_1_HI
        ext.ARG_1_LO
        ext.ARG_2_HI
        ext.ARG_2_LO
        ext.ARG_3_HI
        ext.ARG_3_LO
        ext.RES_HI
        ext.RES_LO
        ext.INST
    )
    ;; source columns
    (
        (* [hub.stack/STACK_ITEM_VALUE_HI 1]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 1]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 2]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 2]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 3]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 3]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 4]     (hub-into-ext-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_LO 4]     (hub-into-ext-activation-flag))
        (*  hub.stack/INSTRUCTION                (hub-into-ext-activation-flag))
    )
)
