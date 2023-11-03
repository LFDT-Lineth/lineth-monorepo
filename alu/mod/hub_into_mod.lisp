;; (defun (alu-mod-activation-flag) (and hub.ALU_MOD_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception

;; (deflookup hub-into-mod
;;     ;reference columns
;;     (
;;         mod.ARG_1_HI
;;         mod.ARG_1_LO
;;         mod.ARG_2_HI
;;         mod.ARG_2_LO
;;         mod.RES_HI
;;         mod.RES_LO
;;         mod.INST
;;     )
;;     ;source columns
;;     (
;;         (* hub.VAL_HI_1     (alu-mod-activation-flag))   ;; arg1
;;         (* hub.VAL_LO_1     (alu-mod-activation-flag))
;;         (* hub.VAL_HI_3     (alu-mod-activation-flag))   ;; arg2
;;         (* hub.VAL_LO_3     (alu-mod-activation-flag))
;;         (* hub.VAL_HI_4     (alu-mod-activation-flag))   ;; res
;;         (* hub.VAL_LO_4     (alu-mod-activation-flag))
;;         (* hub.INSTRUCTION  (alu-mod-activation-flag))
;;     )
;; )
