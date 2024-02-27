(defun (sel-txnData-to-rlpAddr)
  txnData.IS_DEP)

(deflookup 
  txnData-into-rlpAddr
  ;; target columns
  (
    rlpAddr.ADDR_HI
    rlpAddr.ADDR_LO
    rlpAddr.DEP_ADDR_HI
    rlpAddr.DEP_ADDR_LO
    rlpAddr.NONCE
    rlpAddr.RECIPE_1
  )
  ;; source columns
  (
    (* txnData.FROM_HI (sel-txnData-to-rlpAddr))
    (* txnData.FROM_LO (sel-txnData-to-rlpAddr))
    (* txnData.TO_HI (sel-txnData-to-rlpAddr))
    (* txnData.TO_LO (sel-txnData-to-rlpAddr))
    (* txnData.NONCE (sel-txnData-to-rlpAddr))
    (* txnData.IS_DEP (sel-txnData-to-rlpAddr))
  ))


