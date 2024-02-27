(defun (shf-activation-flag)
  (and (unexceptional-stack-row)
       hub_v2.stack/SHF_FLAG))

(deflookup hub-into-shf
    ;; target columns
    (
        shf.ARG_1_HI
        shf.ARG_1_LO
        shf.ARG_2_HI
        shf.ARG_2_LO
        shf.RES_HI
        shf.RES_LO
        shf.INST
    )
    ;; source columns
    (
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1]     (shf-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1]     (shf-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 3]     (shf-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 3]     (shf-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4]     (shf-activation-flag))
        (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4]     (shf-activation-flag))
        (*  hub_v2.stack/INSTRUCTION                (shf-activation-flag))
    )
)
