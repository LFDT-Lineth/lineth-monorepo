(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                            ;;;;
;;;;    X.Y Transient consistency constraints   ;;;;
;;;;                                            ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (tcp_full_address) (+ (* (^ 256 16) tcp_ADDRESS_HI)
                             tcp_ADDRESS_LO)) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;    X.Y.Z Constraints for tcp_FIRST and tcp_AGAIN   ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; TODO: remove when we migrate to the unified permutation argument
(defconstraint transient-consistency---FIRST-AGAIN---precisely-one-is-active-on-every-transient-row-and-both-vanish-otherwise ()
               (eq!   tcp_PEEK_AT_TRANSIENT
                      (+  tcp_FIRST_IN_TXN
                          tcp_AGAIN_IN_TXN
                          )))

(defconstraint    transient-consistency---FIRST-AGAIN---first-transient-row ()
                  (if-zero    (force-bin      (prev tcp_PEEK_AT_TRANSIENT))
                              (eq!   tcp_PEEK_AT_TRANSIENT
                                     tcp_FIRST_IN_TXN)))

(defun   (transient-consistency---repeat-transient-row)    (*    (prev    tcp_PEEK_AT_TRANSIENT)   tcp_PEEK_AT_TRANSIENT))

(defconstraint    transient-consistency---FIRST-AGAIN---repeat-transient-row---change-in-address-transient-storage-key-or-transaction
                  (:guard   (transient-consistency---repeat-transient-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not (remained-constant!   (tcp_full_address)  )  (eq!  tcp_FIRST_IN_TXN  1))
                    (if-not (remained-constant!   tcp_STORAGE_KEY_HI  )  (eq!  tcp_FIRST_IN_TXN  1))
                    (if-not (remained-constant!   tcp_STORAGE_KEY_LO  )  (eq!  tcp_FIRST_IN_TXN  1))
                    (if-not (remained-constant!   tcp_TOTL_TXN_NUMBER )  (eq!  tcp_FIRST_IN_TXN  1))
                    ))

(defconstraint    transient-consistency---FIRST-AGAIN---repeat-transient-row---no-change-in-address-transient-storage-key-or-transaction
                  (:guard   (transient-consistency---repeat-transient-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if (remained-constant!   (tcp_full_address))
                           (if (remained-constant!   tcp_STORAGE_KEY_HI)
                                    (if (remained-constant!   tcp_STORAGE_KEY_LO)
                                             (if (remained-constant!   tcp_TOTL_TXN_NUMBER)
                                                      (eq!  tcp_AGAIN_IN_TXN  1))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    X.Y.Z Consistency constraints   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    transient-consistency---first-encounter-with-transient-slot---initial-value-is-zero
                  (:guard   tcp_FIRST_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (vanishes!   tcp_VALUE_CURR_HI)
                    (vanishes!   tcp_VALUE_CURR_LO)
                    ))

(defconstraint    transient-consistency---repeat-encounter-with-transient-slot---linking-constraints
                  (:guard   tcp_AGAIN_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   tcp_VALUE_CURR_HI   (prev tcp_VALUE_NEXT_HI))
                    (eq!   tcp_VALUE_CURR_LO   (prev tcp_VALUE_NEXT_LO))
                    ))
