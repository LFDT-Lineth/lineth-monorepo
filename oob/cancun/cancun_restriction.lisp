(module oob)

(defconstraint cancun-restriction ()
    (vanishes! (flag-sum-eip-bls12-precompiles)))