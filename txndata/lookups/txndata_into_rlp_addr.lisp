(defun   (txn-data-into-rlp-addr-selector)   (*   txndata.USER
                                                  txndata.HUB
                                                  txndata.hub/IS_DEPLOYMENT))

(defclookup
  txndata-into-rlp-addr
  ;; target columns
  (
   rlpaddr.ADDR_HI
   rlpaddr.ADDR_LO
   rlpaddr.DEP_ADDR_HI
   rlpaddr.DEP_ADDR_LO
   rlpaddr.NONCE
   rlpaddr.RECIPE_1
   )
  ;; source selector
  (txn-data-into-rlp-addr-selector)
  ;; source columns
  (
   txndata.hub/FROM_ADDRESS_HI
   txndata.hub/FROM_ADDRESS_LO
   txndata.hub/TO_ADDRESS_HI
   txndata.hub/TO_ADDRESS_LO
   txndata.hub/NONCE
   RLP_ADDR_RECIPE_1
   )
  )

