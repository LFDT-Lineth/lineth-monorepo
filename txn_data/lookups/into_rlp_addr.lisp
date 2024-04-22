(defun (sel-txnData-to-rlpaddr)
  txnData.IS_DEP)

(deflookup 
  txnData-into-rlpaddr
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
    (* txnData.FROM_HI (sel-txnData-to-rlpaddr))
    (* txnData.FROM_LO (sel-txnData-to-rlpaddr))
    (* txnData.TO_HI (sel-txnData-to-rlpaddr))
    (* txnData.TO_LO (sel-txnData-to-rlpaddr))
    (* txnData.NONCE (sel-txnData-to-rlpaddr))
    (* txnData.IS_DEP (sel-txnData-to-rlpaddr))
  ))


