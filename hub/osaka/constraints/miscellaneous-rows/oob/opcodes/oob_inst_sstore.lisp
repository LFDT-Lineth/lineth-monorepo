(module hub)


(defun (set-OOB-instruction---sstore    kappa               ;; offset
                                        gas_actual          ;; GAS_ACTUAL
                                        ) (begin
                                        (eq! (shift    misc/OOB_INST      kappa) OOB_INST_SSTORE )
                                        ;; (eq! (shift    (misc_oob_data_1)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_2)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_3)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                        (eq! (shift    (misc_oob_data_5)    kappa) gas_actual)
                                        ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                        ))
