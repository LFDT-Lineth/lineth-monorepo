(module rlptxn)

(defun (is-integer-phase)
  (force-bin (+
               ;; IS_RLP_PREFIX
               IS_CHAIN_ID
               IS_NONCE
               IS_GAS_PRICE
               IS_MAX_PRIORITY_FEE_PER_GAS
               IS_MAX_FEE_PER_GAS
               IS_GAS_LIMIT
               ;; IS_TO
               IS_VALUE
               ;; IS_DATA
               ;; IS_ACCESS_LIST
               ;; IS_BETA
               IS_Y
               IS_R
               IS_S
               )))

(defun (phase-appropriate-integer-hi)
  (+   (* IS_R (next cmp/EXO_DATA_1))
       (* IS_S (next cmp/EXO_DATA_1))))

(defun (phase-appropriate-integer-lo)
  (+   (* IS_CHAIN_ID                   txn/CHAIN_ID                  )
       (* IS_NONCE                      txn/NONCE                     )
       (* IS_GAS_PRICE                  txn/GAS_PRICE                 )
       (* IS_MAX_PRIORITY_FEE_PER_GAS   txn/MAX_PRIORITY_FEE_PER_GAS  )
       (* IS_MAX_FEE_PER_GAS            txn/MAX_FEE_PER_GAS           )
       (* IS_GAS_LIMIT                  txn/GAS_LIMIT                 )
       (* IS_VALUE                      txn/VALUE                     )
       (* IS_Y                          Y_PARITY                      )
       (* IS_R                          (next cmp/EXO_DATA_2)         )
       (* IS_S                          (next cmp/EXO_DATA_2)         )
       ))


(defun    (is-first-row-of-integer-phase)    (* (is-integer-phase) TXN))

(defconstraint    integer-phase-constraint
                  (:guard    (is-first-row-of-integer-phase))
                  (rlp-compound-constraint---INTEGER   1
                                                       (phase-appropriate-integer-hi)
                                                       (phase-appropriate-integer-lo)
                                                       1))
