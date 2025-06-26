(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;   9.2 MISC/MXP instruction for MSIZE   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (set-MXP-instruction---for-MSIZE   kappa)
  (eq!
    (shift misc/MXP_INST kappa) EVM_INST_MSIZE))
