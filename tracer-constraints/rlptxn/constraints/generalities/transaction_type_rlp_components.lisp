(module rlptxn)

(defun   (RLP-components-of-type-0-transactions)
  (+
    IS_RLP_PREFIX
    ;; IS_CHAIN_ID
    IS_NONCE
    IS_GAS_PRICE
    ;; IS_MAX_PRIORITY_FEE_PER_GAS
    ;; IS_MAX_FEE_PER_GAS
    IS_GAS_LIMIT
    IS_TO
    IS_VALUE
    IS_DATA
    ;; IS_ACCESS_LIST
    IS_BETA
    ;; IS_Y
    IS_R
    IS_S
    ))

(defun   (RLP-components-of-type-1-transactions)
  (+
    IS_RLP_PREFIX
    IS_CHAIN_ID
    IS_NONCE
    IS_GAS_PRICE
    ;; IS_MAX_PRIORITY_FEE_PER_GAS
    ;; IS_MAX_FEE_PER_GAS
    IS_GAS_LIMIT
    IS_TO
    IS_VALUE
    IS_DATA
    IS_ACCESS_LIST
    ;; IS_BETA
    IS_Y
    IS_R
    IS_S
    ))

(defun   (RLP-components-of-type-2-transactions)
  (+
    IS_RLP_PREFIX
    IS_CHAIN_ID
    IS_NONCE
    ;; IS_GAS_PRICE
    IS_MAX_PRIORITY_FEE_PER_GAS
    IS_MAX_FEE_PER_GAS
    IS_GAS_LIMIT
    IS_TO
    IS_VALUE
    IS_DATA
    IS_ACCESS_LIST
    ;; IS_BETA
    IS_Y
    IS_R
    IS_S
    ))

(defproperty  admissible-RLP-components-for-type-0-transactions  (if  (==  TYPE_0  1)  (eq!  (RLP-components-of-type-0-transactions)  1)))
(defproperty  admissible-RLP-components-for-type-1-transactions  (if  (==  TYPE_1  1)  (eq!  (RLP-components-of-type-1-transactions)  1)))
(defproperty  admissible-RLP-components-for-type-2-transactions  (if  (==  TYPE_2  1)  (eq!  (RLP-components-of-type-2-transactions)  1)))
