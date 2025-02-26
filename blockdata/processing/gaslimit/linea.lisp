(module blockdata)

(defconst GAS_LIMIT_MINIMUM LINEA_GAS_LIMIT_MINIMUM)
(defconst GAS_LIMIT_MAXIMUM LINEA_GAS_LIMIT_MAXIMUM)

(defconstraint block-gas-limit-value (:guard IS_GASLIMIT)
    (eq! BLOCK_GAS_LIMIT LINEA_BLOCK_GAS_LIMIT))
