(defun (ext-activation-flag)
  (and (unexceptional-stack-row)
       hub_v2.stack/EXT_FLAG))

(deflookup hub-into-alu-ext
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
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 3]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 3]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 2]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 2]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4]     (ext-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4]     (ext-activation-flag))
        (*  hub_v2.stack/INSTRUCTION                (ext-activation-flag))
    )
)
