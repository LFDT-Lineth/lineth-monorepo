(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X.Y MISC/MXP CALL-type instructions   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (set-MXP-instruction---for-CALL-type   kappa       ;; row offset kappa
                                              instruction ;; instruction
                                              cdo_hi      ;; call data offset high
                                              cdo_lo      ;; call data offset low
                                              cds_hi      ;; call data size high
                                              cds_lo      ;; call data size low
                                              r@o_hi      ;; return at offset high
                                              r@o_lo      ;; return at offset low
                                              r@c_hi      ;; return at capacity high
                                              r@c_lo      ;; return at capacity low
                                              )
  (begin
    (eq! (shift misc/MXP_INST        kappa) instruction )
    (eq! (shift misc/MXP_OFFSET_1_HI kappa) cdo_hi      )
    (eq! (shift misc/MXP_OFFSET_1_LO kappa) cdo_lo      )
    (eq! (shift misc/MXP_SIZE_1_HI   kappa) cds_hi      )
    (eq! (shift misc/MXP_SIZE_1_LO   kappa) cds_lo      )
    (eq! (shift misc/MXP_OFFSET_2_HI kappa) r@o_hi      )
    (eq! (shift misc/MXP_OFFSET_2_LO kappa) r@o_lo      )
    (eq! (shift misc/MXP_SIZE_2_HI   kappa) r@c_hi      )
    (eq! (shift misc/MXP_SIZE_2_LO   kappa) r@c_lo      )
    ))
