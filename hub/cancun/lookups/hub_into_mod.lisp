(defun (hub-into-mod-activation-flag)
  (* (unexceptional-stack-row)
      hub.stack/MOD_FLAG))

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
        (* [hub.stack/STACK_ITEM_VALUE_HI 1]     (hub-into-mod-activation-flag))   ;; arg1
        (* [hub.stack/STACK_ITEM_VALUE_LO 1]     (hub-into-mod-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 2]     (hub-into-mod-activation-flag))   ;; arg2
        (* [hub.stack/STACK_ITEM_VALUE_LO 2]     (hub-into-mod-activation-flag))
        (* [hub.stack/STACK_ITEM_VALUE_HI 4]     (hub-into-mod-activation-flag))   ;; res
        (* [hub.stack/STACK_ITEM_VALUE_LO 4]     (hub-into-mod-activation-flag))
        (*  hub.stack/INSTRUCTION                (hub-into-mod-activation-flag))
    )
)
