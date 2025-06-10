(defun (hub-into-gas-trigger) (* hub.PEEK_AT_STACK hub.CMC))

(deflookup hub-into-gas
           ;; target columns
	   (
	     gas.IOMF
	     gas.GAS_ACTUAL
	     gas.GAS_COST
	     gas.EXCEPTIONS_AHOY
	     gas.OUT_OF_GAS_EXCEPTION
           )
           ;; source columns
	   (
	                          (hub-into-gas-trigger)
	     (* hub.GAS_ACTUAL    (hub-into-gas-trigger))
	     (* hub.GAS_COST      (hub-into-gas-trigger))
	     (* hub.XAHOY         (hub-into-gas-trigger))
	     (* hub.stack/OOGX    (hub-into-gas-trigger))
           )
)
