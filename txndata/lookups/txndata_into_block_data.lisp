(defun   (source-selector---TXNDATA-into-BLOCKDATA)   txndata.HUB)
(defun   (target-selector---TXNDATA-into-BLOCKDATA)   blockdata.IOMF) ;; ""

;; recall that REL_BLOCK starts at 1 for the first block in the conflation
;; thus the actual block number is FIRST + (REL - 1) along non padding rows

(defclookup
  txndata-into-block-data
  ; target selector
  (target-selector---TXNDATA-into-BLOCKDATA)
  ; target columns
  (
   blockdata.REL_BLOCK
   blockdata.NUMBER
   blockdata.TIMESTAMP
   blockdata.BASEFEE
   blockdata.BLOCK_GAS_LIMIT
   blockdata.COINBASE_HI
   blockdata.COINBASE_LO
   )
  ; source selector
  (source-selector---TXNDATA-into-BLOCKDATA)
  ; source columns
  (
   txndata.BLK_NUMBER
   txndata.hub/btc_BLOCK_NUMBER
   txndata.hub/btc_TIMESTAMP
   txndata.hub/btc_BASEFEE
   txndata.hub/btc_BLOCK_GAS_LIMIT
   txndata.hub/btc_COINBASE_ADDRESS_HI
   txndata.hub/btc_COINBASE_ADDRESS_LO
   )
  )



