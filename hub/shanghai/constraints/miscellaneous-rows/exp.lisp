(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   9.1 MISC/EXP constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (set-EXP-instruction-exp-log
         kappa          ;; row offset
         exponent_hi    ;; exponent high
         exponent_lo    ;; exponent low
         ) (begin
         (eq! (shift  misc/EXP_INST    kappa)    EXP_INST_EXPLOG)
         (eq! (shift [misc/EXP_DATA 1] kappa)    exponent_hi )
         (eq! (shift [misc/EXP_DATA 2] kappa)    exponent_lo )))

(defun (set-EXP-instruction-MODEXP-lead-log
         kappa          ;; row offset
         raw_lead_hi    ;; raw leading word where exponent starts, high part
         raw_lead_lo    ;; raw leading word where exponent starts, low  part
         cds_cutoff     ;; min{max{cds - 96 - bbs, 0}, 32}
         ebs_cutoff     ;; min{ebs, 32}
         ) (begin
         (eq! (shift  misc/EXP_INST    kappa)    EXP_INST_MODEXPLOG)
         (eq! (shift [misc/EXP_DATA 1] kappa)    raw_lead_hi )
         (eq! (shift [misc/EXP_DATA 2] kappa)    raw_lead_lo )
         (eq! (shift [misc/EXP_DATA 3] kappa)    cds_cutoff  )
         (eq! (shift [misc/EXP_DATA 4] kappa)    ebs_cutoff  )
         ))
