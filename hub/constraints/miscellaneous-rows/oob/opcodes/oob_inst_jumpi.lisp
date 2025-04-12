(module hub)


(defun (set-OOB-instruction---jumpi    kappa               ;; offset
                                       pc_new_hi           ;; high part of proposed new program counter
                                       pc_new_lo           ;; low  part of proposed new program counter
                                       jump_condition_hi   ;; high part of jump condition
                                       jump_condition_lo   ;; low  part of jump condition
                                       code_size           ;; code size of byte code currently executing
                                       ) (begin
                                       (eq! (shift     misc/OOB_INST          kappa) OOB_INST_JUMPI)
                                       (eq! (shift    (misc_oob_data_1)       kappa) pc_new_hi)
                                       (eq! (shift    (misc_oob_data_2)       kappa) pc_new_lo)
                                       (eq! (shift    (misc_oob_data_3)       kappa) jump_condition_hi)
                                       (eq! (shift    (misc_oob_data_4)       kappa) jump_condition_lo)
                                       (eq! (shift    (misc_oob_data_5)       kappa) code_size)
                                       ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                       ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                       ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                       ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                       ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                       ))
