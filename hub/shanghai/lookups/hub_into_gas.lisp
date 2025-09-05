(defun (hub-into-gas-trigger) (* hub.PEEK_AT_STACK hub.CMC))

(defclookup hub-into-gas
  ;; target columns
  (
   gas.GAS_ACTUAL
   gas.GAS_COST
   gas.XAHOY
   gas.OOGX
  )
  ;; source selector
  (hub-into-gas-trigger)
  ;; source columns
  (
   hub.GAS_ACTUAL
   hub.GAS_COST
   hub.XAHOY
   hub.stack/OOGX
   )
)
