(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X.Y MISC/MXP constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (set-MXP-instruction---for-MCOPY   kappa             ;; row offset kappa
                                          target_offset_hi  ;; target offset hi
                                          target_offset_lo  ;; target offset lo
                                          source_offset_hi  ;; source offset hi
                                          source_offset_lo  ;; source offset lo
                                          size_hi           ;; size hi
                                          size_lo           ;; size lo
                                          )
  (begin
    (eq! (shift misc/MXP_INST        kappa) EVM_INST_MCOPY   )
    (eq! (shift misc/MXP_OFFSET_1_HI kappa) target_offset_hi )
    (eq! (shift misc/MXP_OFFSET_1_LO kappa) target_offset_lo )
    (eq! (shift misc/MXP_SIZE_1_HI   kappa) size_hi          )
    (eq! (shift misc/MXP_SIZE_1_LO   kappa) size_lo          )
    (eq! (shift misc/MXP_OFFSET_2_HI kappa) source_offset_hi )
    (eq! (shift misc/MXP_OFFSET_2_LO kappa) source_offset_lo )
    (eq! (shift misc/MXP_SIZE_2_HI   kappa) size_hi          )
    (eq! (shift misc/MXP_SIZE_2_LO   kappa) size_lo          )))

