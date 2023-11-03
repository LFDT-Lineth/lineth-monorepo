(defun (sel-txnData-to-romLex)
  (* txnData.IS_DEP (~ txnData.INIT_CODE_SIZE)))

(deflookup 
  txnData-into-romLex
  ;reference columns
  (
    romLex.CODE_FRAGMENT_INDEX
    romLex.CODE_SIZE
    romLex.ADDR_HI
    romLex.ADDR_LO
    romLex.DEP_NUMBER
    romLex.DEP_STATUS
  )
  ;source columns
  (
    (* txnData.CODE_FRAGMENT_INDEX (sel-txnData-to-romLex))
    (* txnData.INIT_CODE_SIZE (sel-txnData-to-romLex))
    (* txnData.TO_HI (sel-txnData-to-romLex))
    (* txnData.TO_LO (sel-txnData-to-romLex))
    (sel-txnData-to-romLex)
    (sel-txnData-to-romLex)
  ))


