(defun (sel-txndata-to-romLex)
  (* txndata.IS_DEP (~ txndata.INIT_CODE_SIZE)))

(deflookup 
  txndata-into-romLex
  ;; target columns
  (
    romlex.CODE_FRAGMENT_INDEX
    romlex.CODE_SIZE
    romlex.ADDRESS_HI
    romlex.ADDRESS_LO
    romlex.DEPLOYMENT_NUMBER
    romlex.DEPLOYMENT_STATUS
  )
  ;; source columns
  (
    (* txndata.CODE_FRAGMENT_INDEX (sel-txndata-to-romLex))
    (* txndata.INIT_CODE_SIZE (sel-txndata-to-romLex))
    (* txndata.TO_HI (sel-txndata-to-romLex))
    (* txndata.TO_LO (sel-txndata-to-romLex))
    (sel-txndata-to-romLex)
    (sel-txndata-to-romLex)
  ))


