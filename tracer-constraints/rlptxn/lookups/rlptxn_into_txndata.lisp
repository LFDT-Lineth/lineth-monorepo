(defun    (src-selector---rlp-txn---into---txn-data)    rlptxn.TXN   ) ;;
(defun    (tgt-selector---rlp-txn---into---txn-data)    txndata.USER ) ;; ""
(defun    (txn-data-hub-view-cfi)       (shift   txndata.hub/CFI   -1))

(defclookup
  (rlp-txn-rcpt---into---txn-data  :unchecked)
  ;; target selector: none
  (tgt-selector---rlp-txn---into---txn-data)
  ;; target columns
  (
   txndata.USER
   txndata.RLP
   txndata.USER_TXN_NUMBER
   txndata.prover___USER_TXN_NUMBER_MAX
   (txn-data-hub-view-cfi)
   txndata.rlp/REQUIRES_EVM_EXECUTION
   txndata.rlp/IS_DEPLOYMENT
   txndata.rlp/TX_TYPE
   txndata.rlp/TO_ADDRESS_HI
   txndata.rlp/TO_ADDRESS_LO
   txndata.rlp/VALUE
   txndata.rlp/NONCE
   txndata.rlp/NUMBER_OF_ZERO_BYTES
   txndata.rlp/NUMBER_OF_NONZERO_BYTES
   txndata.rlp/NUMBER_OF_ACCESS_LIST_ADDRESSES
   txndata.rlp/NUMBER_OF_ACCESS_LIST_STORAGE_KEYS
   txndata.rlp/GAS_LIMIT
   txndata.rlp/GAS_PRICE
   txndata.rlp/MAX_PRIORITY_FEE_PER_GAS
   txndata.rlp/MAX_FEE_PER_GAS
   txndata.rlp/LENGTH_OF_DELEGATION_LIST
   )
  ;; source selector
  (src-selector---rlp-txn---into---txn-data)
  ;; source columns
  (
   1
   1
   rlptxn.USER_TXN_NUMBER
   rlptxn.prover___USER_TXN_NUMBER_MAX
   rlptxn.CODE_FRAGMENT_INDEX
   rlptxn.REQUIRES_EVM_EXECUTION
   rlptxn.IS_DEPLOYMENT
   rlptxn.txn/TX_TYPE
   rlptxn.txn/TO_HI
   rlptxn.txn/TO_LO
   rlptxn.txn/VALUE
   rlptxn.txn/NONCE
   rlptxn.txn/NUMBER_OF_ZERO_BYTES
   rlptxn.txn/NUMBER_OF_NONZERO_BYTES
   rlptxn.txn/NUMBER_OF_PREWARMED_ADDRESSES
   rlptxn.txn/NUMBER_OF_PREWARMED_STORAGE_KEYS
   rlptxn.txn/GAS_LIMIT
   rlptxn.txn/GAS_PRICE
   rlptxn.txn/MAX_PRIORITY_FEE_PER_GAS
   rlptxn.txn/MAX_FEE_PER_GAS
   rlptxn.LENGTH_OF_DELEGATION_LIST
   )
  )


