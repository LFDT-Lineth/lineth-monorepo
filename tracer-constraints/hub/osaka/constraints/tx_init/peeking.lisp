(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   X   TX_INIT phase               ;;
;;   X.Y Setting the peeking flags   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    tx-init---setting-peeking-flags---unconditionally-set-the-first-few-peeking-flags
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (+    (shift    PEEK_AT_TRANSACTION     tx-init---row-offset---TXN                    )
                                (shift    PEEK_AT_MISCELLANEOUS   tx-init---row-offset---MISC                   )
                                (shift    PEEK_AT_ACCOUNT         tx-init---row-offset---ACC---coinbase-warming )
                                )
                          (+  tx-init---row-offset---ACC---coinbase-warming  1)
                          ))

(defconstraint    tx-init---setting-peeking-flags---transaction-failure
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-failure-prediction)
                                  (eq!    (+    (shift    PEEK_AT_TRANSACTION      tx-init---row-offset---TXN                                        )
                                                (shift    PEEK_AT_MISCELLANEOUS    tx-init---row-offset---MISC                                       )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---coinbase-warming                     )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---sender-pay-for-gas                   )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---sender-value-transfer                )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---recipient-value-reception            )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---delegate-reading                     )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---sender-value-transfer---undoing      )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---recipient-value-reception---undoing  )
                                                (shift    PEEK_AT_CONTEXT          tx-init---row-offset---CON---context-initialization-row---failure )
                                                )
                                          tx-init---non-stack-rows---failure
                                          )))

(defconstraint    tx-init---setting-peeking-flags---transaction-success
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-success-prediction)
                                  (eq!    (+    (shift    PEEK_AT_TRANSACTION      tx-init---row-offset---TXN                                        )
                                                (shift    PEEK_AT_MISCELLANEOUS    tx-init---row-offset---MISC                                       )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---coinbase-warming                     )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---sender-pay-for-gas                   )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---sender-value-transfer                )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---recipient-value-reception            )
                                                (shift    PEEK_AT_ACCOUNT          tx-init---row-offset---ACC---delegate-reading                     )
                                                (shift    PEEK_AT_CONTEXT          tx-init---row-offset---CON---context-initialization-row---success )
                                                )
                                          tx-init---non-stack-rows---success
                                          )))

(defconstraint    tx-init---justifying-predictions---transaction-failure
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-failure-prediction)
                                  (begin
                                    (eq!    (tx-init---transaction-failure-prediction)    (shift    CONTEXT_WILL_REVERT    tx-init---row-offset---first-execution-phase-row---failure))
                                    (eq!    (tx-init---transaction-end-stamp)             (shift    CONTEXT_REVERT_STAMP   tx-init---row-offset---first-execution-phase-row---failure)))))

(defconstraint    tx-init---justifying-predictions---transaction-success
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-success-prediction)
                                  (begin
                                    (eq!    (tx-init---transaction-failure-prediction)    (shift    CONTEXT_WILL_REVERT    tx-init---row-offset---first-execution-phase-row---success))
                                    (eq!    (tx-init---transaction-end-stamp)             (shift    CONTEXT_REVERT_STAMP   tx-init---row-offset---first-execution-phase-row---success)))))
