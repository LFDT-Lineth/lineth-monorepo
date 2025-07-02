(defun (sel-txndata-to-rlpaddr)
  txndata.IS_DEP)

(defclookup
  txndata-into-rlpaddr
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
  (sel-txndata-to-rlpaddr)
  ;; source columns
  (
    txndata.FROM_HI
    txndata.FROM_LO
    txndata.TO_HI
    txndata.TO_LO
    txndata.NONCE
    txndata.IS_DEP
  ))


