(defun (add-activation-flag)
  (and (unexceptional-stack-row) hub_v2.stack/ADD_FLAG))

(deflookup 
  hub-into-add
  ;; target columns
  (
    add.ARG_1_HI
    add.ARG_1_LO
    add.ARG_2_HI
    add.ARG_2_LO
    add.RES_HI
    add.RES_LO
    add.INST
  )
  ;; source columns
  (
    (* [hub_v2.stack/STACK_ITEM_VALUE_HI 1] (add-activation-flag)) ;; arg1

    (* [hub_v2.stack/STACK_ITEM_VALUE_LO 1] (add-activation-flag))
    (* [hub_v2.stack/STACK_ITEM_VALUE_HI 2] (add-activation-flag)) ;; arg2

    (* [hub_v2.stack/STACK_ITEM_VALUE_LO 2] (add-activation-flag))
    (* [hub_v2.stack/STACK_ITEM_VALUE_HI 4] (add-activation-flag)) ;; res

    (* [hub_v2.stack/STACK_ITEM_VALUE_LO 4] (add-activation-flag))
    (* hub_v2.stack/INSTRUCTION (add-activation-flag))
  ))


