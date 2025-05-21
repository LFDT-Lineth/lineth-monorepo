(module hub)


(defun (set-OOB-instruction---blake-params    kappa                            ;; offset
                                              call_gas                         ;; call gas i.e. gas provided to the precompile
                                              blake_r                          ;; rounds parameter of the call data of BLAKE2f
                                              blake_f                          ;; f      parameter of the call data of BLAKE2f ("final block indicator")
                                              ) (begin
                                              (eq! (shift     misc/OOB_INST       kappa) OOB_INST_BLAKE_PARAMS )
                                              (eq! (shift    (misc_oob_data_1)    kappa) call_gas )
                                              ;; (eq! (shift    (misc_oob_data_2)    kappa) )
                                              ;; (eq! (shift    (misc_oob_data_3)    kappa) )
                                              ;; (eq! (shift    (misc_oob_data_4)    kappa) )
                                              ;; (eq! (shift    (misc_oob_data_5)    kappa) )
                                              (eq! (shift    (misc_oob_data_6)    kappa) blake_r )
                                              (eq! (shift    (misc_oob_data_7)    kappa) blake_f )
                                              ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                              ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                              ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                              ))
