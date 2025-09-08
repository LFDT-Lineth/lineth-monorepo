(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. USER transaction processing    ;;
;;    X.Y RLP shorthands                ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (USER-transaction---RLP---tx-type)                           (shift   rlp/TX_TYPE                              ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---to-address-hi-or-zero)             (shift   rlp/TO_ADDRESS_HI                        ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---to-address-lo-or-zero)             (shift   rlp/TO_ADDRESS_LO                        ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---is-deployment)                     (shift   rlp/IS_DEPLOYMENT                        ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---nonce)                             (shift   rlp/NONCE                                ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---value)                             (shift   rlp/VALUE                                ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---number-of-zero-bytes)              (shift   rlp/NUMBER_OF_ZERO_BYTES                 ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---number-of-nonzero-bytes)           (shift   rlp/NUMBER_OF_NONZERO_BYTES              ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---gas-limit)                         (shift   rlp/GAS_LIMIT                            ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---gas-price)                         (shift   rlp/GAS_PRICE                            ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---max-priority-fee)                  (shift   rlp/MAX_PRIORITY_FEE_PER_GAS             ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---max-fee)                           (shift   rlp/MAX_FEE_PER_GAS                      ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---number-of-access-list-keys)        (shift   rlp/NUMBER_OF_ACCESS_LIST_STORAGE_KEYS   ROFF___USER___RLP_ROW))
(defun   (USER-transaction---RLP---number-of-access-list-addresses)   (shift   rlp/NUMBER_OF_ACCESS_LIST_ADDRESSES      ROFF___USER___RLP_ROW))

(defun   (USER-transaction---RLP---is-message-call)    (-   1    (USER-transaction---RLP---is-deployment)))

(defun   (USER-transaction---payload-size)             (+   (USER-transaction---RLP---number-of-zero-bytes)
                                                            (USER-transaction---RLP---number-of-nonzero-bytes)))
(defun   (USER-transaction---weighted-byte-count)      (+   (*   1    (USER-transaction---RLP---number-of-zero-bytes))
                                                            (*   4    (USER-transaction---RLP---number-of-nonzero-bytes))))

(defun   (USER-transaction---payload-cost)             (*   STANDARD_TOKEN_COST       (USER-transaction---weighted-byte-count)))
(defun   (USER-transaction---payload-floor-cost)       (*   FLOOR_TOKEN_COST          (USER-transaction---weighted-byte-count)))
(defun   (USER-transaction---transaction-floor-cost)   (+   GAS_CONST_G_TRANSACTION   (USER-transaction---payload-floor-cost)))

