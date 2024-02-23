;; (defun (word-comparison-module-activation-flag) (and hub.WORD_COMPARISON_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception
;;
;; (deflookup hub-into-wcp
;;     ;reference columns
;;     (
;;         wcp.ARG_1_HI
;;         wcp.ARG_1_LO
;;         wcp.ARG_2_HI
;;         wcp.ARG_2_LO
;;         wcp.RES_HI
;;         wcp.RES_LO
;;         wcp.INST
;;     )
;;     ;source columns
;;     (
;;         (* hub.VAL_HI_1     (word-comparison-module-activation-flag))
;;         (* hub.VAL_LO_1     (word-comparison-module-activation-flag))
;;         (* hub.VAL_HI_3     (word-comparison-module-activation-flag))
;;         (* hub.VAL_LO_3     (word-comparison-module-activation-flag))
;;         (* hub.VAL_HI_4     (word-comparison-module-activation-flag))
;;         (* hub.VAL_LO_4     (word-comparison-module-activation-flag))
;;         (* hub.INSTRUCTION  (word-comparison-module-activation-flag))
;;     )
;; )
