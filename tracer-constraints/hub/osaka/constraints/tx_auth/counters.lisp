(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X Authorization phase     ;;
;;   X.Y Counters              ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (successful-sender-is-authority-delegation)   (*  auth/AUTHORIZATION_TUPLE_IS_VALID  auth/SENDER_IS_AUTHORITY ) )
(defun   (prev-tx-auth)   (prev TX_AUTH))
(defun   (curr-tx-auth)         TX_AUTH)
(defun   (first-tx-auth)   (*  (-  1  (prev-tx-auth)) (curr-tx-auth))) ;; ""
(defun   (again-tx-auth)   (*         (prev-tx-auth)  (curr-tx-auth)))

(defconst
  ROFF___PREVIOUS_AUTH_ROW_IS_REMOTE___AUTH_ROW  -2
  ROFF___PREVIOUS_AUTH_ROW_IS_REMOTE___ACC_ROW   -1

  ROFF___PREVIOUS_AUTH_ROW_IS_NEARBY___AUTH_ROW  -1
  )


(defconstraint    authorization-phase---counters---initialize         (:guard  (first-tx-auth))
                  (begin
                    (eq!   auth/TUPLE_INDEX               1                                           )
                    (eq!   auth/SENDER_IS_AUTHORITY_ACC   (successful-sender-is-authority-delegation) )
                    ))

(defconstraint    authorization-phase---counters---link-1-row-back
                  (:guard  (again-tx-auth))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (prev   PEEK_AT_AUTHORIZATION)
                                 (if-not-zero   PEEK_AT_AUTHORIZATION
                                                (begin
                                                  (eq!   auth/TUPLE_INDEX
                                                         (+ (shift   auth/TUPLE_INDEX   ROFF___PREVIOUS_AUTH_ROW_IS_NEARBY___AUTH_ROW)
                                                            1))
                                                  (eq!   auth/SENDER_IS_AUTHORITY_ACC
                                                         (+ (shift   auth/SENDER_IS_AUTHORITY_ACC    ROFF___PREVIOUS_AUTH_ROW_IS_NEARBY___AUTH_ROW)
                                                            (successful-sender-is-authority-delegation))
                                                         )))))

(defconstraint    authorization-phase---counters---link-2-rows-back
                  (:guard  (again-tx-auth))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero   (prev   PEEK_AT_ACCOUNT)
                                 (if-not-zero   PEEK_AT_AUTHORIZATION
                                                (begin
                                                  (eq!   auth/TUPLE_INDEX
                                                         (+ (shift   auth/TUPLE_INDEX    ROFF___PREVIOUS_AUTH_ROW_IS_REMOTE___AUTH_ROW)
                                                            1))
                                                  (eq!   auth/SENDER_IS_AUTHORITY_ACC
                                                         (+ (shift   auth/SENDER_IS_AUTHORITY_ACC    ROFF___PREVIOUS_AUTH_ROW_IS_REMOTE___AUTH_ROW)
                                                            (successful-sender-is-authority-delegation))
                                                         )))))
