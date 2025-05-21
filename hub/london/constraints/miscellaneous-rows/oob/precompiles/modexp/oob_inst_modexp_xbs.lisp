(module hub)


(defun (set-OOB-instruction---modexp-xbs    kappa                            ;; offset
                                            xbs_hi                           ;; high part of some {b,e,m}bs
                                            xbs_lo                           ;; low  part of some {b,e,m}bs
                                            ybs_lo                           ;; low  part of some {b,e,m}bs
                                            compute_max                      ;; bit indicating whether to compute max(xbs, ybs) or not
                                            ) (begin
                                            (eq! (shift     misc/OOB_INST       kappa) OOB_INST_MODEXP_XBS )
                                            (eq! (shift    (misc_oob_data_1)    kappa) xbs_hi      )
                                            (eq! (shift    (misc_oob_data_2)    kappa) xbs_lo      )
                                            (eq! (shift    (misc_oob_data_3)    kappa) ybs_lo      )
                                            (eq! (shift    (misc_oob_data_4)    kappa) compute_max )
                                            ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                            ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                            ))
