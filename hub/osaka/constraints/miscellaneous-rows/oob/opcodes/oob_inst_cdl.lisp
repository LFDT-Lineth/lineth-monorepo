(module hub)


(defun (set-OOB-instruction---cdl    kappa               ;; row offset
                                     offset_hi           ;; offset within call data, high part
                                     offset_lo           ;; offset within call data, low  part
                                     call_data_size      ;; call data size
                                     ) (begin
                                     (eq! (shift     misc/OOB_INST       kappa) OOB_INST_CDL )
                                     (eq! (shift    (misc_oob_data_1)    kappa) offset_hi)
                                     (eq! (shift    (misc_oob_data_2)    kappa) offset_lo)
                                     ;; (eq! (shift    (misc_oob_data_3)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                     (eq! (shift    (misc_oob_data_5)    kappa) call_data_size)
                                     ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                     ))

