(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. USER transaction processing    ;;
;;    X.Y Common computations           ;;
;;    X.Y.Z EIP-2681 max nonce check    ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    USER-transaction---common-computations---EIP-2681-max-nonce-upper-bound-check
                  (:guard   (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (small-call-to-LT      ROFF___USER___CMPTN_ROW___EIP-2681___MAX_NONCE_UPPER_BOUND_CHECK
                                           (USER-transaction---RLP---nonce)
                                           EIP2681_MAX_NONCE)
                    (result-must-be-true   ROFF___USER___CMPTN_ROW___EIP-2681___MAX_NONCE_UPPER_BOUND_CHECK)
                    ))

