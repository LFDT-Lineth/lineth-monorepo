(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.Z USER_TXN_NUMBER constraints    ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    user-txn-number-constraints---vanishes-initially     (:domain {0})  (vanishes!             USER_TXN_NUMBER)) ;; ""
(defconstraint    user-txn-number-constraints---increments             ()             (will-inc!             USER_TXN_NUMBER   (*  (next USER)  (next HUB))))
(defproperty      user-txn-number-constraints---zero-one-increments                   (has-0-1-increments    USER_TXN_NUMBER))

