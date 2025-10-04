(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X. USER transaction processing     ;;
;;    X.Y Common computations            ;;
;;    X.Y.Z Detecting empty call data    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint    USER-transaction-processing---common-computations---detecting-empty-call-data
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (small-call-to-ISZERO    ROFF___USER___CMPTN_ROW___DETECTING_EMPTY_CALL_DATA
                                           (USER-transaction---payload-size)))

(defun    (USER-transaction---nonzero-data-size)   (- 1 (shift  computation/WCP_RES  ROFF___USER___CMPTN_ROW___DETECTING_EMPTY_CALL_DATA)))


(defconstraint    USER-transaction-processing---common-computations---setting-COPY_TXCD---message-call-case
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (USER-transaction---RLP---is-message-call)
                                  (eq!   (USER-transaction---HUB---copy-txcd)
                                         (*   (USER-transaction---HUB---requires-EVM-execution)
                                              (USER-transaction---nonzero-data-size)))))

(defconstraint    USER-transaction-processing---common-computations---setting-COPY_TXCD---deployment-transaction-case
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (USER-transaction---RLP---is-deployment)
                                  (eq!   (USER-transaction---HUB---copy-txcd)    0)))

(defconstraint    USER-transaction-processing---common-computations---setting-REQUIRES_EVM_EXECTION---deployment-transaction-case
                  (:guard    (first-row-of-USER-transaction))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (USER-transaction---RLP---is-deployment)
                                  (eq!   (USER-transaction---HUB---requires-EVM-execution)
                                         (USER-transaction---nonzero-data-size))))

