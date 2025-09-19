(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;;    X. USER transaction processing                     ;;
;;    X.Y Common computations                            ;;
;;    X.Y.Z Gas limit must cover the upfront gas cost    ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---common-computations---gas-limit-must-cover-the-upfront-gas-cost---comparison
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (small-call-to-LEQ    ROFF___USER___CMPTN_ROW___GAS_LIMIT_MUST_COVER_THE_UPFRONT_GAS_COST
                                        (USER-transaction---upfront-gas-cost)
                                        (USER-transaction---RLP---gas-limit))
                  )


(defconstraint    USER-transaction---common-computations---gas-limit-must-cover-the-upfront-gas-cost---result-must-be-true
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (result-must-be-true    ROFF___USER___CMPTN_ROW___GAS_LIMIT_MUST_COVER_THE_UPFRONT_GAS_COST)
                  )

(defun    (USER-transaction---upfront-gas-cost)
  (+    (USER-transaction---payload-cost)
        (USER-transaction---init-code-cost)
        GAS_CONST_G_TRANSACTION
        (*  (USER-transaction---RLP---is-deployment)                    GAS_CONST_G_TX_CREATE           )
        (*  (USER-transaction---RLP---number-of-access-list-addresses)  GAS_CONST_G_ACCESS_LIST_ADRESS  )
        (*  (USER-transaction---RLP---number-of-access-list-keys)       GAS_CONST_G_ACCESS_LIST_STORAGE )
        ))

