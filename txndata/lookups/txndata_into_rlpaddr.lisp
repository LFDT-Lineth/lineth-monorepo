(defun (sel-txndata-to-rlpaddr)
  txndata.IS_DEP)

(deflookup
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
  ;; source columns
  (
    (* txndata.FROM_HI (sel-txndata-to-rlpaddr))
    (* txndata.FROM_LO (sel-txndata-to-rlpaddr))
    (* txndata.TO_HI (sel-txndata-to-rlpaddr))
    (* txndata.TO_LO (sel-txndata-to-rlpaddr))
    (* txndata.NONCE (sel-txndata-to-rlpaddr))
    (* txndata.IS_DEP (sel-txndata-to-rlpaddr))
  ))


