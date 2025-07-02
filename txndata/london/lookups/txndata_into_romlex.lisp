(defun (sel-txn-data-to-rom-lex)
  (* txndata.IS_DEP (~ txndata.INIT_CODE_SIZE)))

(defclookup
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
  ;; source selector
  (sel-txn-data-to-rom-lex)
  ;; source columns
  (
    txndata.CODE_FRAGMENT_INDEX
    txndata.INIT_CODE_SIZE
    txndata.TO_HI
    txndata.TO_LO
    1
    1
  ))


