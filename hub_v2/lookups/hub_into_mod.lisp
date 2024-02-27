(defun (mod-activation-flag)
  (and (unexceptional-stack-row)
       hub_v2.stack/MOD_FLAG))

(deflookup hub-into-mod
    ;; target columns
    (
        mod.ARG_1_HI
        mod.ARG_1_LO
        mod.ARG_2_HI
        mod.ARG_2_LO
        mod.RES_HI
        mod.RES_LO
        mod.INST
    )
    ;; source columns
    (
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1]     (mod-activation-flag))   ;; arg1
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1]     (mod-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 3]     (mod-activation-flag))   ;; arg2
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 3]     (mod-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4]     (mod-activation-flag))   ;; res
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4]     (mod-activation-flag))
        (*  hub_v2.stack/INSTRUCTION                (mod-activation-flag))
    )
)
