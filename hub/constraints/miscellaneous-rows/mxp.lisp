(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   9.2 MISC/MXP constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (set-MXP-instruction-type-1 kappa)
  (eq! (shift misc/MXP_INST kappa) EVM_INST_MSIZE))

(defun (set-MXP-instruction-type-2 kappa       ;; row offset kappa
                                   instruction ;; instruction
                                   offset_hi   ;; source offset high
                                   offset_lo)  ;; source offset low
  (begin (eq! (shift misc/MXP_INST kappa) instruction)
         (eq! (shift misc/MXP_OFFSET_1_HI kappa) offset_hi)
         (eq! (shift misc/MXP_OFFSET_1_LO kappa) offset_lo)))

(defun (set-MXP-instruction-type-3 kappa      ;; row offset kappa
                                   offset_hi  ;; source offset high
                                   offset_lo) ;; source offset low
  (begin (eq! (shift misc/MXP_INST kappa) EVM_INST_MSTORE8)
         (eq! (shift misc/MXP_OFFSET_1_HI kappa) offset_hi)
         (eq! (shift misc/MXP_OFFSET_1_LO kappa) offset_lo)))

(defun (set-MXP-instruction-type-4 kappa       ;; row offset kappa
                                   instruction ;; instruction
                                   deploys     ;; bit modifying the behaviour of RETURN pricing
                                   offset_hi   ;; offset high
                                   offset_lo   ;; offset low
                                   size_hi     ;; size high
                                   size_lo)    ;; size low
  (begin (eq! (shift misc/MXP_INST kappa) instruction)
         (eq! (shift misc/MXP_DEPLOYS kappa) deploys)
         (eq! (shift misc/MXP_OFFSET_1_HI kappa) offset_hi)
         (eq! (shift misc/MXP_OFFSET_1_LO kappa) offset_lo)
         (eq! (shift misc/MXP_SIZE_1_HI kappa) size_hi)
         (eq! (shift misc/MXP_SIZE_1_LO kappa) size_lo)))

(defun (set-mxp-instruction-type-5 kappa       ;; row offset kappa
                                   instruction ;; instruction
                                   cdo_hi      ;; call data offset high
                                   cdo_lo      ;; call data offset low
                                   cds_hi      ;; call data size high
                                   cds_lo      ;; call data size low
                                   r@o_hi      ;; return at offset high
                                   r@o_lo      ;; return at offset low
                                   r@c_hi      ;; return at capacity high
                                   r@c_lo)     ;; return at capacity low
  (begin (eq! (shift misc/MXP_INST kappa) instruction)
         (eq! (shift misc/MXP_OFFSET_1_HI kappa) cdo_hi)
         (eq! (shift misc/MXP_OFFSET_1_LO kappa) cdo_lo)
         (eq! (shift misc/MXP_SIZE_1_HI kappa) cds_hi)
         (eq! (shift misc/MXP_SIZE_1_LO kappa) cds_lo)
         (eq! (shift misc/MXP_OFFSET_2_HI kappa) r@o_hi)
         (eq! (shift misc/MXP_OFFSET_2_LO kappa) r@o_lo)
         (eq! (shift misc/MXP_SIZE_2_HI kappa) r@c_hi)
         (eq! (shift misc/MXP_SIZE_2_LO kappa) r@c_lo)))

