(module hub)


(defun (set-OOB-instruction---create    kappa              ;; offset
                                        value_hi           ;; value   (high part)
                                        value_lo           ;; value   (low  part, stack argument of CALL-type instruction)
                                        balance            ;; balance (from caller account)
                                        nonce              ;; callee's nonce
                                        has_code           ;; callee's HAS_CODE
                                        call_stack_depth   ;; current call stack depth
                                        creator_nonce      ;; creator account nonce
                                        ;; init_code_size     ;; init code size (it's necessarily small at this point, so only low part required)
                                        ) (begin
                                        (eq!    (shift     misc/OOB_INST       kappa)   OOB_INST_CREATE  )
                                        (eq!    (shift    (misc_oob_data_1)    kappa)   value_hi         )
                                        (eq!    (shift    (misc_oob_data_2)    kappa)   value_lo         )
                                        (eq!    (shift    (misc_oob_data_3)    kappa)   balance          )
                                        (eq!    (shift    (misc_oob_data_4)    kappa)   nonce            )
                                        (eq!    (shift    (misc_oob_data_5)    kappa)   has_code         )
                                        (eq!    (shift    (misc_oob_data_6)    kappa)   call_stack_depth )
                                        ;; (eq!    (shift    (misc_oob_data_7)    kappa) )    ;; value_is_nonzero
                                        ;; (eq!    (shift    (misc_oob_data_8)    kappa) )    ;; aborting condition
                                        (eq!    (shift    (misc_oob_data_9)    kappa)   creator_nonce    )
                                        ;; (eq!    (shift    (misc_oob_data_10)   kappa)   init_code_size   ) ;; XXXXXX
                                        ))

