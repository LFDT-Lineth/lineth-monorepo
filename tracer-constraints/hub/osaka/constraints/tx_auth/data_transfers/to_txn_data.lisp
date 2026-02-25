(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X Authorization phase     ;;
;;   X.Y Value transfers       ;;
;;   X.Y.Z To RLP_AUTH         ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF___VALUE_TRANSFER_TO_TXN_ROW___REMOTE_AUTH_ROW  -2
  ROFF___VALUE_TRANSFER_TO_TXN_ROW___NEARBY_AUTH_ROW  -1
  ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW           0

  ROFF___VALUE_TRANSFER_TO_TXN_ROW___ONE_ROW_REMOVED  -1
  )

(defun   (final-row-of-TX_AUTH-phase)   (*  (shift  TX_AUTH              ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW)
                                            (shift  PEEK_AT_TRANSACTION  ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW)))


(defconstraint    authorization-phase---data-transfer---nearby-case   (:guard   (final-row-of-TX_AUTH-phase))
                  (if-not-zero  (shift  PEEK_AT_AUTHORIZATION  ROFF___VALUE_TRANSFER_TO_TXN_ROW___ONE_ROW_REMOVED)
                                (begin
                                  (eq!  (shift  transaction/LENGTH_OF_DELEGATION_LIST                ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW ) (shift  auth/TUPLE_INDEX              ROFF___VALUE_TRANSFER_TO_TXN_ROW___NEARBY_AUTH_ROW ))
                                  (eq!  (shift  transaction/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS  ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW ) (shift  auth/SENDER_IS_AUTHORITY_ACC  ROFF___VALUE_TRANSFER_TO_TXN_ROW___NEARBY_AUTH_ROW ))
                                  )))


(defconstraint    authorization-phase---data-transfer---remote-case
                  (:guard   (final-row-of-TX_AUTH-phase))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero  (shift  PEEK_AT_ACCOUNT        ROFF___VALUE_TRANSFER_TO_TXN_ROW___ONE_ROW_REMOVED)
                                (begin
                                  (eq!  (shift  PEEK_AT_AUTHORIZATION                                ROFF___VALUE_TRANSFER_TO_TXN_ROW___REMOTE_AUTH_ROW ) 1)
                                  (eq!  (shift  transaction/LENGTH_OF_DELEGATION_LIST                ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW         ) (shift  auth/TUPLE_INDEX              ROFF___VALUE_TRANSFER_TO_TXN_ROW___REMOTE_AUTH_ROW ))
                                  (eq!  (shift  transaction/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS  ROFF___VALUE_TRANSFER_TO_TXN_ROW___TXN_ROW         ) (shift  auth/SENDER_IS_AUTHORITY_ACC  ROFF___VALUE_TRANSFER_TO_TXN_ROW___REMOTE_AUTH_ROW ))
                                  )))

