(defun (hub-into-gas-trigger) (* hub.PEEK_AT_STACK hub.CMC))

(defclookup hub-into-gas
  ;; target columns
  (
   gas.IOMF
   gas.GAS_ACTUAL
   gas.GAS_COST
   gas.EXCEPTIONS_AHOY
   gas.OUT_OF_GAS_EXCEPTION
  )
  ;; source selector
  (hub-into-gas-trigger)
  ;; source columns
  (
   1
   hub.GAS_ACTUAL
   hub.GAS_COST
   hub.XAHOY
   hub.stack/OOGX
   )
)
