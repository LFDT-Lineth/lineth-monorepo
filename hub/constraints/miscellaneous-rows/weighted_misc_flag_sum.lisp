(module hub)


(defun (weighted-MISC-flag-sum kappa) (+ (*  MISC_WEIGHT_EXP  (shift misc/EXP_FLAG kappa))
                                         (*  MISC_WEIGHT_MMU  (shift misc/MMU_FLAG kappa))
                                         (*  MISC_WEIGHT_MXP  (shift misc/MXP_FLAG kappa))
                                         (*  MISC_WEIGHT_OOB  (shift misc/OOB_FLAG kappa))
                                         (*  MISC_WEIGHT_STP  (shift misc/STP_FLAG kappa))))
