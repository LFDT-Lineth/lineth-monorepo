(defun (sel-txn-data-to-rom-lex)
  (* txndata.IS_DEP (~ txndata.INIT_CODE_SIZE)))

(deflookup
  txndata-into-romlex
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
    (* txndata.CODE_FRAGMENT_INDEX (sel-txn-data-to-rom-lex))
    (* txndata.INIT_CODE_SIZE (sel-txn-data-to-rom-lex))
    (* txndata.TO_HI (sel-txn-data-to-rom-lex))
    (* txndata.TO_LO (sel-txn-data-to-rom-lex))
    (sel-txn-data-to-rom-lex)
    (sel-txn-data-to-rom-lex)
  ))


