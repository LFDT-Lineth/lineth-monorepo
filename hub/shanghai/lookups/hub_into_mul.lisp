(defun (hub-into-mul-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/MUL_FLAG))

(deflookup hub-into-mul
    ;; target columns
    (
        mul.ARG_1_HI
        mul.ARG_1_LO
        mul.ARG_2_HI
        mul.ARG_2_LO
        mul.RES_HI
        mul.RES_LO
        mul.INST
    )
    ;; source columns
    (
        (* [hub.stack/STACK_ITEM_VALUE_HI 1]     (hub-into-mul-activation-flag))   ;; arg1
        (* [hub.stack/STACK_ITEM_VALUE_LO 1]     (hub-into-mul-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 2]     (hub-into-mul-activation-flag))   ;; arg2
        (* [hub.stack/STACK_ITEM_VALUE_LO 2]     (hub-into-mul-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 4]     (hub-into-mul-activation-flag))   ;; res
        (* [hub.stack/STACK_ITEM_VALUE_LO 4]     (hub-into-mul-activation-flag))
        (*  hub.stack/INSTRUCTION                (hub-into-mul-activation-flag))
    )
)
