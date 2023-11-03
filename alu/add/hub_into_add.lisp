;; (defun (alu-add-activation-flag) (and hub.ALU_ADD_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception


;; (deflookup hub-into-alu-add
;;     ;reference columns
;;     (
;;         add.ARG_1_HI
;;         add.ARG_1_LO
;;         add.ARG_2_HI
;;         add.ARG_2_LO
;;         add.RES_HI
;;         add.RES_LO
;;         add.INST
;;     )
;;     ;source columns
;;     (
;;         (* hub.VAL_HI_1     (alu-add-activation-flag))   ;; arg1
;;         (* hub.VAL_LO_1     (alu-add-activation-flag))
;;         (* hub.VAL_HI_3     (alu-add-activation-flag))   ;; arg2
;;         (* hub.VAL_LO_3     (alu-add-activation-flag))
;;         (* hub.VAL_HI_4     (alu-add-activation-flag))   ;; res
;;         (* hub.VAL_LO_4     (alu-add-activation-flag))
;;         (* hub.INSTRUCTION  (alu-add-activation-flag))
;;     )
;; )
