(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. USER transaction processing    ;;
;;    X.Y Transaction decoding          ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (USER-transaction---is-type-0)   (shift    rlp/TYPE_0   ROFF___USER___RLP_ROW))
(defun   (USER-transaction---is-type-1)   (shift    rlp/TYPE_1   ROFF___USER___RLP_ROW))
(defun   (USER-transaction---is-type-2)   (shift    rlp/TYPE_2   ROFF___USER___RLP_ROW))
(defun   (USER-transaction---is-type-3)   (shift    rlp/TYPE_3   ROFF___USER___RLP_ROW))
(defun   (USER-transaction---is-type-4)   (shift    rlp/TYPE_4   ROFF___USER___RLP_ROW))



(defun    (USER-transaction---tx-decoding---tx-type-with-fixed-gas-price)
  (+
    (USER-transaction---is-type-0)
    (USER-transaction---is-type-1)
    ;; (USER-transaction---is-type-2)
    ;; (USER-transaction---is-type-3)
    ;; (USER-transaction---is-type-4)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-sans-fixed-gas-price)
  (+
    ;; (USER-transaction---is-type-0)
    ;; (USER-transaction---is-type-1)
    (USER-transaction---is-type-2)
    (USER-transaction---is-type-3)
    (USER-transaction---is-type-4)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-sans-access-set)
  (+
    (USER-transaction---is-type-0)
    ;; (USER-transaction---is-type-1)
    ;; (USER-transaction---is-type-2)
    ;; (USER-transaction---is-type-3)
    ;; (USER-transaction---is-type-4)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-with-access-set)
  (+
    ;; (USER-transaction---is-type-0)
    (USER-transaction---is-type-1)
    (USER-transaction---is-type-2)
    (USER-transaction---is-type-3)
    (USER-transaction---is-type-4)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-sans-delegation)
  (+
    (USER-transaction---is-type-0)
    (USER-transaction---is-type-1)
    (USER-transaction---is-type-2)
    (USER-transaction---is-type-3)
    ;; (USER-transaction---is-type-4)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-with-delegation)
  (+
    ;; (USER-transaction---is-type-0)
    ;; (USER-transaction---is-type-1)
    ;; (USER-transaction---is-type-2)
    ;; (USER-transaction---is-type-3)
    (USER-transaction---is-type-4)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-flag-sum)
  (+
    (USER-transaction---tx-decoding---tx-type-with-fixed-gas-price)
    (USER-transaction---tx-decoding---tx-type-sans-fixed-gas-price)
    ))

(defun    (USER-transaction---tx-decoding---tx-type-wght-sum)
  (+
    (*  0  (USER-transaction---is-type-0))
    (*  1  (USER-transaction---is-type-1))
    (*  2  (USER-transaction---is-type-2))
    (*  3  (USER-transaction---is-type-3))
    (*  4  (USER-transaction---is-type-4))
    ))


(defconstraint   USER-transaction---transaction-decoding---precisely-one-of-the-transaction-type-flags-lights-up
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (USER-transaction---tx-decoding---tx-type-flag-sum)   1))

(defconstraint   USER-transaction---transaction-decoding---the-transaction-type-flags-decode-the-transaction-type
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (USER-transaction---RLP---tx-type)
                        (USER-transaction---tx-decoding---tx-type-wght-sum)))

(defconstraint   USER-transaction---transaction-decoding---transactions-sans-access-list-have-no-addresses-nor-storage-keys-to-prewarm
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (USER-transaction---tx-decoding---tx-type-sans-access-set)
                                (begin
                                  (vanishes!   (USER-transaction---RLP---number-of-access-list-keys))
                                  (vanishes!   (USER-transaction---RLP---number-of-access-list-addresses))
                                  )))

