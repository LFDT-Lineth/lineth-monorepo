(module oob)

;; TODO: disable for Prague
(defconstraint cancun-restriction ()
    (vanishes! (flag-sum-eip-bls12-precompiles)))
