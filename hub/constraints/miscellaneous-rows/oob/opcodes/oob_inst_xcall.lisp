(module hub)


(defun (set-OOB-instruction---xcall    kappa           ;; offset
                                       value_hi        ;; value (high part)
                                       value_lo        ;; value (low  part, stack argument of CALL-type instruction)
                                       ) (begin
                                       (eq!    (shift     misc/OOB_INST       kappa)   OOB_INST_XCALL )
                                       (eq!    (shift    (misc_oob_data_1)    kappa)   value_hi       )
                                       (eq!    (shift    (misc_oob_data_2)    kappa)   value_lo       )
                                       ;; (eq!    (shift    (misc_oob_data_3)    kappa) )
                                       ;; (eq!    (shift    (misc_oob_data_4)    kappa) )
                                       ;; (eq!    (shift    (misc_oob_data_5)    kappa) )
                                       ;; (eq!    (shift    (misc_oob_data_6)    kappa) )
                                       ;; (eq!    (shift    (misc_oob_data_7)    kappa) )    ;; value_is_nonzero
                                       ;; (eq!    (shift    (misc_oob_data_8)    kappa) )    ;; value_is_zero    ... I don't remember why I ask for both ...
                                       ;; (eq!    (shift    (misc_oob_data_9)    kappa) )
                                       ;; (eq!    (shift    (misc_oob_data_10)   kappa) )
                                       ))
