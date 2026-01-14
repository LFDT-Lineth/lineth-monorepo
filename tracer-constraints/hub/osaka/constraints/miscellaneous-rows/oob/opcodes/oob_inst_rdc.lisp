(module hub)


(defun (set-OOB-instruction---rdc    kappa                   ;; row offset
                                     source_offset_hi        ;; offset within call data, high part
                                     source_offset_lo        ;; offset within call data, low  part
                                     size_hi                 ;; size of data to copy, high part
                                     size_lo                 ;; size of data to copy, low  part
                                     return_data_size        ;; return data size
                                     ) (begin
                                     (eq! (shift     misc/OOB_INST       kappa) OOB_INST_RDC)
                                     (eq! (shift    (misc_oob_data_1)    kappa) source_offset_hi)
                                     (eq! (shift    (misc_oob_data_2)    kappa) source_offset_lo)
                                     (eq! (shift    (misc_oob_data_3)    kappa) size_hi)
                                     (eq! (shift    (misc_oob_data_4)    kappa) size_lo)
                                     (eq! (shift    (misc_oob_data_5)    kappa) return_data_size)
                                     ;; (eq! (shift    (misc_oob_data_6)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_7)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_8)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_9)    kappa) )
                                     ;; (eq! (shift    (misc_oob_data_10)   kappa) )
                                     ))
