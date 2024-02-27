(defun (bin-activation-flag)
  (and (unexceptional-stack-row)
       hub_v2.stack/BIN_FLAG))

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
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1]     (bin-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1]     (bin-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 3]     (bin-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 3]     (bin-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4]     (bin-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4]     (bin-activation-flag))
        (*  hub_v2.stack/INSTRUCTION                (bin-activation-flag))
    )
)
