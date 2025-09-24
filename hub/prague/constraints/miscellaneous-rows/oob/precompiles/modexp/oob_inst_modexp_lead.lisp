(module hub)


(defun (set-OOB-instruction---modexp-lead    kappa                            ;; offset
                                             bbs_lo                           ;; low part of bbs (base     byte size)
                                             cds                              ;; call data size
                                             ebs_lo                           ;; low part of ebs (exponent byte size)
                                             ) (begin
                                             (eq! (shift     misc/OOB_INST       kappa) OOB_INST_MODEXP_LEAD )
                                             (eq! (shift    (misc_oob_data_1)    kappa) bbs_lo )
                                             (eq! (shift    (misc_oob_data_2)    kappa) cds    )
                                             (eq! (shift    (misc_oob_data_3)    kappa) ebs_lo )
                                             ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                             ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                             ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                             ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                             ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                             ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                             ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                             ))
