(module rlptxn)

(defun (about-to-exit-current-phase)
    (force-bin (+
        (* IS_RLP_PREFIX                   (- 1 (next IS_RLP_PREFIX               )))
        (* IS_CHAIN_ID                     (- 1 (next IS_CHAIN_ID                 )))
        (* IS_NONCE                        (- 1 (next IS_NONCE                    )))
        (* IS_GAS_PRICE                    (- 1 (next IS_GAS_PRICE                )))
        (* IS_MAX_PRIORITY_FEE_PER_GAS     (- 1 (next IS_MAX_PRIORITY_FEE_PER_GAS )))
        (* IS_MAX_FEE_PER_GAS              (- 1 (next IS_MAX_FEE_PER_GAS          )))
        (* IS_GAS_LIMIT                    (- 1 (next IS_GAS_LIMIT                )))
        (* IS_TO                           (- 1 (next IS_TO                       )))
        (* IS_VALUE                        (- 1 (next IS_VALUE                    )))
        (* IS_DATA                         (- 1 (next IS_DATA                     )))
        (* IS_ACCESS_LIST                  (- 1 (next IS_ACCESS_LIST              )))
        (* IS_BETA                         (- 1 (next IS_BETA                     )))
        (* IS_Y                            (- 1 (next IS_Y                        )))
        (* IS_R                            (- 1 (next IS_R                        )))
        (* IS_S                            (- 1 (next IS_S                        )))
    )))

(defproperty about-to-exit-current-phase-shorthand-is-binary
             (is-binary   (about-to-exit-current-phase)))

(defcomputedcolumn (PHASE_END :binary) (about-to-exit-current-phase))
