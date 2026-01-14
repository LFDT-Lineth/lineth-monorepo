(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X.Y MISC/MXP CALL-type instructions   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (set-MXP-instruction---single-mxp-offset-instructions   kappa       ;; row offset kappa
                                                               instruction ;; instruction
                                                               deploys     ;; bit modifying the behaviour of RETURN pricing
                                                               offset_hi   ;; offset high
                                                               offset_lo   ;; offset low
                                                               size_hi     ;; size high
                                                               size_lo     ;; size low
                                                               )
  (begin
    (eq! (shift misc/MXP_INST        kappa) instruction )
    (eq! (shift misc/MXP_DEPLOYS     kappa) deploys     )
    (eq! (shift misc/MXP_OFFSET_1_HI kappa) offset_hi   )
    (eq! (shift misc/MXP_OFFSET_1_LO kappa) offset_lo   )
    (eq! (shift misc/MXP_SIZE_1_HI   kappa) size_hi     )
    (eq! (shift misc/MXP_SIZE_1_LO   kappa) size_lo     )
    ))
