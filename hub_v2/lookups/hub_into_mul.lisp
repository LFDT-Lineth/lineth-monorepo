(defun (mul-activation-flag)
  (and (unexceptional-stack-row)
       hub_v2.stack/MUL_FLAG))

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
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1]     (mul-activation-flag))   ;; arg1
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1]     (mul-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 3]     (mul-activation-flag))   ;; arg2
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 3]     (mul-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4]     (mul-activation-flag))   ;; res
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4]     (mul-activation-flag))
        (*  hub_v2.stack/INSTRUCTION                (mul-activation-flag))
    )
)
