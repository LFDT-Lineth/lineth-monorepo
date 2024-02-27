(module hub_v2)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   9.1 MISC/EXP constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (set-exp-inst-exp-log
         kappa
         exponent_hi
         exponent_lo
         ) (begin
         (eq! (shift  misc/EXP_INST    kappa)    exp.EXP_INST_EXPLOG)
         (eq! (shift [misc/EXP_DATA 1] kappa)    exponent_hi )
         (eq! (shift [misc/EXP_DATA 2] kappa)    exponent_lo )))

(defun (set-exp-inst-exp-log
         kappa
         raw_lead_hi
         raw_lead_lo
         cds_cutoff
         ebs_cutoff
         ) (begin
         (eq! (shift  misc/EXP_INST    kappa)    exp.EXP_INST_MODEXPLOG)
         (eq! (shift [misc/EXP_DATA 1] kappa)    raw_lead_hi )
         (eq! (shift [misc/EXP_DATA 2] kappa)    raw_lead_lo )
         (eq! (shift [misc/EXP_DATA 3] kappa)    cds_cutoff  )
         (eq! (shift [misc/EXP_DATA 4] kappa)    ebs_cutoff  )
         ))
