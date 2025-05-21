(module hub)


;; note that the instruction is part of the interface !
(defun (set-OOB-instruction---common    kappa                            ;; offset
                                        common_precompile_oob_inst       ;; relevant OOB instruction
                                        call_gas                         ;; call gas i.e. gas provided to the precompile
                                        cds                              ;; call data size
                                        r@c                              ;; return at capacity
                                        ) (begin
                                        (eq!    (shift     misc/OOB_INST       kappa)    common_precompile_oob_inst )
                                        (eq!    (shift    (misc_oob_data_1)    kappa)    call_gas                   )
                                        (eq!    (shift    (misc_oob_data_2)    kappa)    cds                        )
                                        (eq!    (shift    (misc_oob_data_3)    kappa)    r@c                        )
                                        ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                        ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                        ))
