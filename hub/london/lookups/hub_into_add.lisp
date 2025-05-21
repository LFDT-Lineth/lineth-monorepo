(defun (hub-into-add-activation-flag)
  (* (unexceptional-stack-row) hub.stack/ADD_FLAG))

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
    (* [hub.stack/STACK_ITEM_VALUE_HI 1] (hub-into-add-activation-flag)) ;; arg1
    (* [hub.stack/STACK_ITEM_VALUE_LO 1] (hub-into-add-activation-flag))
    (* [hub.stack/STACK_ITEM_VALUE_HI 2] (hub-into-add-activation-flag)) ;; arg2
    (* [hub.stack/STACK_ITEM_VALUE_LO 2] (hub-into-add-activation-flag))
    (* [hub.stack/STACK_ITEM_VALUE_HI 4] (hub-into-add-activation-flag)) ;; result
    (* [hub.stack/STACK_ITEM_VALUE_LO 4] (hub-into-add-activation-flag))
    (* hub.stack/INSTRUCTION             (hub-into-add-activation-flag)) ;; instruction
  ))


