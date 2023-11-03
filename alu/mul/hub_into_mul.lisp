;; (defun (alu-mul-activation-flag) (and hub.ALU_MUL_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception

;; (deflookup hub-into-alu-mul
;;     ;reference columns
;;     (
;;         mul.ARG_1_HI
;;         mul.ARG_1_LO
;;         mul.ARG_2_HI
;;         mul.ARG_2_LO
;;         mul.RES_HI
;;         mul.RES_LO
;;         mul.INST
;;     )
;;     ;source columns
;;     (
;;         (* hub.VAL_HI_1     (alu-mul-activation-flag))   ;; arg1
;;         (* hub.VAL_LO_1     (alu-mul-activation-flag))
;;         (* hub.VAL_HI_3     (alu-mul-activation-flag))   ;; arg2
;;         (* hub.VAL_LO_3     (alu-mul-activation-flag))
;;         (* hub.VAL_HI_4     (alu-mul-activation-flag))   ;; res
;;         (* hub.VAL_LO_4     (alu-mul-activation-flag))
;;         (* hub.INSTRUCTION  (alu-mul-activation-flag))
;;     )
;; )
