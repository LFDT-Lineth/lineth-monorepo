(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X     TX_INIT phase        ;;
;;   X.Y   Common constraints   ;;
;;   X.Y.Z Miscellaneous row    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   tx-init---setting-miscellaneous-row-flags
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (weighted-MISC-flag-sum   tx-init---row-offset---MISC)
                      (*   MISC_WEIGHT_MMU   (shift transaction/COPY_TXCD   tx-init---row-offset---TXN))))

(defconstraint   tx-init---copying-transaction-call-data
                 (:guard (tx-init---standard-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (shift misc/MMU_FLAG     tx-init---row-offset---MISC)
                                 (set-MMU-instruction---exo-to-ram-transplants    tx-init---row-offset---MISC           ;; offset
                                                                                  ABS_TX_NUM                            ;; source ID
                                                                                  (tx-init---call-data-context-number)  ;; target ID
                                                                                  ;; aux_id                             ;; auxiliary ID
                                                                                  ;; src_offset_hi                      ;; source offset high
                                                                                  ;; src_offset_lo                      ;; source offset low
                                                                                  ;; tgt_offset_lo                      ;; target offset low
                                                                                  (tx-init---call-data-size)            ;; size
                                                                                  ;; ref_offset                         ;; reference offset
                                                                                  ;; ref_size                           ;; reference size
                                                                                  ;; success_bit                        ;; success bit
                                                                                  ;; limb_1                             ;; limb 1
                                                                                  ;; limb_2                             ;; limb 2
                                                                                  EXO_SUM_WEIGHT_TXCD                   ;; weighted exogenous module flag sum
                                                                                  RLP_TXN_PHASE_DATA                    ;; phase
                                                                                  )))
