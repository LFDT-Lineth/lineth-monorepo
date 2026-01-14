(module hub)


(defun (set-OOB-instruction---blake-cds    kappa                            ;; offset
                                           cds                              ;; call data size
                                           r@c                              ;; return at capacity
                                           ) (begin
                                           (eq! (shift     misc/OOB_INST       kappa) OOB_INST_BLAKE_CDS )
                                           ;; (eq! (shift    (misc_oob_data_1)    kappa) )
                                           (eq! (shift    (misc_oob_data_2)    kappa) cds )
                                           (eq! (shift    (misc_oob_data_3)    kappa) r@c )
                                           ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                           ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                           ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                           ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                           ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                           ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                           ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                           ))
