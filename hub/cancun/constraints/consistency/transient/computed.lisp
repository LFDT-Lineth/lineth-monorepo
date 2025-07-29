(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;; FIRST/AGAIN in transaction ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (tcp_FIRST_IN_TXN)
  (fwd-changes-within    tcp_PEEK_AT_TRANSIENT ;; perspective
                         tcp_ADDRESS_HI        ;; columns
                         tcp_ADDRESS_LO
                         tcp_STORAGE_KEY_HI
                         tcp_STORAGE_KEY_LO
                         tcp_TOTL_TXN_NUMBER
                         ))
(defcomputed
  (tcp_AGAIN_IN_TXN)
  (fwd-unchanged-within    tcp_PEEK_AT_TRANSIENT ;; perspective
                           tcp_ADDRESS_HI        ;; columns
                           tcp_ADDRESS_LO
                           tcp_STORAGE_KEY_HI
                           tcp_STORAGE_KEY_LO
                           tcp_TOTL_TXN_NUMBER
                           ))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;; Binary constraints ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    transient-consistency---binarities ()
                  (begin
                    (is-binary   tcp_FIRST_IN_TXN )
                    (is-binary   tcp_AGAIN_IN_TXN )
                    ))
