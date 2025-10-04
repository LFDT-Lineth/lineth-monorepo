(module hub)


(defun (weighted-MISC-flag-sum    rel_offset) (+   (*   MISC_WEIGHT_EXP   (shift misc/EXP_FLAG   rel_offset))
                                                   (*   MISC_WEIGHT_MMU   (shift misc/MMU_FLAG   rel_offset))
                                                   (*   MISC_WEIGHT_MXP   (shift misc/MXP_FLAG   rel_offset))
                                                   (*   MISC_WEIGHT_OOB   (shift misc/OOB_FLAG   rel_offset))
                                                   (*   MISC_WEIGHT_STP   (shift misc/STP_FLAG   rel_offset))))

(defun (weighted-MISC-flag-sum-sans-MMU    rel_offset) (+   (*   MISC_WEIGHT_EXP   (shift misc/EXP_FLAG   rel_offset))
                                                            ;; (*   MISC_WEIGHT_MMU   (shift misc/MMU_FLAG   rel_offset))
                                                            (*   MISC_WEIGHT_MXP   (shift misc/MXP_FLAG   rel_offset))
                                                            (*   MISC_WEIGHT_OOB   (shift misc/OOB_FLAG   rel_offset))
                                                            (*   MISC_WEIGHT_STP   (shift misc/STP_FLAG   rel_offset))))
