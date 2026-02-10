(module rlptxn)

(defun    (is-first-row-of-TO-phase)    (* IS_TO TXN))

(defconstraint TO-phase-constraints ()
               (if-not-zero (is-first-row-of-TO-phase)
                            (if-not-zero IS_DEPLOYMENT
                                         ;; deployment transaction case
                                         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                         (begin
                                           ;; can't be 7702-txs
                                           (vanishes! TYPE_4)
                                           ;; setting CT_MAX
                                           (debug  (vanishes!  (shift   CT_MAX   1)))
                                           ;; writing the RLP prefix to the RLP string
                                           (set-limb   1
                                                       (*  RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)) ;; ""
                                                       1)
                                           ;; setting PHASE_END
                                           (eq!   (shift PHASE_END 1)   1))
                                         ;; message call transaction case
                                         ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                                         (begin
                                           ;; calling the RLP_UTILS instruction
                                           (rlp-compound-constraint---ADDRESS    1
                                                                                 txn/TO_HI
                                                                                 txn/TO_LO)
                                           ;; setting PHASE_END
                                           (eq! (shift PHASE_END 3) 1))
                                         )))
