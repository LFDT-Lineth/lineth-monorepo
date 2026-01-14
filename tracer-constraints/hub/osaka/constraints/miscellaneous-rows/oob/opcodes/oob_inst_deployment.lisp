(module hub)


(defun (set-OOB-instruction---deployment    kappa                            ;; offset
                                            code_size_hi                     ;; code size hi
                                            code_size_lo                     ;; code size lo
                                            ) (begin
                                            (eq! (shift     misc/OOB_INST       kappa)   OOB_INST_DEPLOYMENT )
                                            (eq! (shift    (misc_oob_data_1)    kappa)   code_size_hi)
                                            (eq! (shift    (misc_oob_data_2)    kappa)   code_size_lo)
                                            ;; (eq! (shift    (misc_oob_data_3)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_7)    kappa) )    ;; max code size exception
                                            ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                            ))
