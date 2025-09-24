(module hub)


(defun (set-OOB-instruction---modexp-extract    kappa                            ;; offset
                                                cds                              ;; call data size
                                                bbs_lo                           ;; low part of bbs (base     byte size)
                                                ebs_lo                           ;; low part of ebs (exponent byte size)
                                                mbs_lo                           ;; low part of mbs (modulus  byte size)
                                                ) (begin
                                                (eq!    (shift     misc/OOB_INST       kappa) OOB_INST_MODEXP_EXTRACT )
                                                ;; (eq! (shift    (misc_oob_data_1)    kappa) )
                                                (eq!    (shift    (misc_oob_data_2)    kappa) cds    )
                                                (eq!    (shift    (misc_oob_data_3)    kappa) bbs_lo )
                                                (eq!    (shift    (misc_oob_data_4)    kappa) ebs_lo )
                                                (eq!    (shift    (misc_oob_data_5)    kappa) mbs_lo )
                                                ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                                ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                                ))
