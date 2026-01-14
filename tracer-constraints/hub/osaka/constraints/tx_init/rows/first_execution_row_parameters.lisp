(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   X     TX_INIT phase            ;;
;;   X.Y   Common constraints       ;;
;;   X.Y.Z First row of execution   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    tx-init---setting-parameters-on-the-first-row-of-new-context---failure
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-failure-prediction)
                                  (first-row-of-new-context
                                    tx-init---row-offset---first-execution-phase-row---failure                                             ;; row offset
                                    0                                                                                                      ;; next caller context number
                                    (shift   account/CODE_FRAGMENT_INDEX           tx-init---row-offset---ACC---recipient-value-reception) ;; next CFI
                                    (shift   transaction/GAS_INITIALLY_AVAILABLE   tx-init---row-offset---TXN)                             ;; initially available gas
                                    )))

(defconstraint    tx-init---setting-parameters-on-the-first-row-of-new-context---success
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-success-prediction)
                                  (first-row-of-new-context
                                    tx-init---row-offset---first-execution-phase-row---success                                             ;; row offset
                                    0                                                                                                      ;; next caller context number
                                    (shift   account/CODE_FRAGMENT_INDEX           tx-init---row-offset---ACC---recipient-value-reception) ;; next CFI
                                    (shift   transaction/GAS_INITIALLY_AVAILABLE   tx-init---row-offset---TXN)                             ;; initially available gas
                                    )))
