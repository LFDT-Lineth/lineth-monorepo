(defun (oob-into-bls-ref-table-activation-flag)
  oob.BLS_REF_TABLE_FLAG)

(defclookup
  (oob-into-bls-ref-table :unchecked)
  ;; target columns
  (
    blsreftable.PRC_NAME
    blsreftable.NUM_INPUTS
    blsreftable.DISCOUNT
  )
  ;; source selector
  (oob-into-bls-ref-table-activation-flag)
  ;; source columns
  (
    oob.OUTGOING_INST
    [oob.OUTGOING_DATA 1]
    oob.OUTGOING_RES_LO
  ))

