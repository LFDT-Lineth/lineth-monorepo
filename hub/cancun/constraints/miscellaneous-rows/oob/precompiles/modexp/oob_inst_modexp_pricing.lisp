(module hub)


(defun (set-OOB-instruction---modexp-pricing    kappa                            ;; offset
                                                call_gas                         ;; call gas i.e. gas provided to the precompile
                                                r@c                              ;; return at capacity
                                                exponent_log                     ;; leading (≤) word log of exponent
                                                max_mbs_bbs                      ;; call data size
                                                ) (begin
                                                (eq! (shift     misc/OOB_INST       kappa) OOB_INST_MODEXP_PRICING )
                                                (eq! (shift    (misc_oob_data_1)    kappa) call_gas )
                                                ;; (eq! (shift    (misc_oob_data_2)    kappa) )
                                                (eq! (shift    (misc_oob_data_3)    kappa) r@c )
                                                ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                                (eq! (shift    (misc_oob_data_6)    kappa) exponent_log )
                                                (eq! (shift    (misc_oob_data_7)    kappa) max_mbs_bbs )
                                                ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                                ))
