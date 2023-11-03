;; (defun (alu-ext-activation-flag) (and hub.ALU_EXT_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception

;; (deflookup hub-into-alu-ext
;;     ;reference columns
;;     (
;;         ext.ARG_1_HI
;;         ext.ARG_1_LO
;;         ext.ARG_2_HI
;;         ext.ARG_2_LO
;;         ext.ARG_3_HI
;;         ext.ARG_3_LO
;;         ext.RES_HI
;;         ext.RES_LO
;;         ext.INST
;;     )
;;     ;source columns
;;     (
;;         (* hub.VAL_HI_1     (alu-ext-activation-flag))   ; arg1
;;         (* hub.VAL_LO_1     (alu-ext-activation-flag))   ;
;;         (* hub.VAL_HI_3     (alu-ext-activation-flag))   ; arg2
;;         (* hub.VAL_LO_3     (alu-ext-activation-flag))   ;
;;         (* hub.VAL_HI_2     (alu-ext-activation-flag))   ; arg3
;;         (* hub.VAL_LO_2     (alu-ext-activation-flag))   ;
;;         (* hub.VAL_HI_4     (alu-ext-activation-flag))   ; res
;;         (* hub.VAL_LO_4     (alu-ext-activation-flag))   ;
;;         (* hub.INSTRUCTION  (alu-ext-activation-flag))
;;     )
;; )
