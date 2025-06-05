(module blockdata)

(defconst (GAS_LIMIT_ENABLE :binary :extern) 1)
(defconst (GAS_LIMIT_MINIMUM :i64 :extern) LINEA_GAS_LIMIT_MINIMUM)
(defconst (GAS_LIMIT_MAXIMUM :i64 :extern) LINEA_GAS_LIMIT_MAXIMUM)

(defconstraint block-gas-limit-value (:guard (* GAS_LIMIT_ENABLE IS_GASLIMIT))
    (eq! BLOCK_GAS_LIMIT LINEA_BLOCK_GAS_LIMIT))
