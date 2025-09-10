(module rlptxn)

(defun (persp-flag-sum) (force-bin (+ TXN CMP)))

(defun (about-to-enter-new-phase)
    (force-bin (+
        (* (- 1 IS_RLP_PREFIX               )     (next IS_RLP_PREFIX               ))
        (* (- 1 IS_CHAIN_ID                 )     (next IS_CHAIN_ID                 ))
        (* (- 1 IS_NONCE                    )     (next IS_NONCE                    ))
        (* (- 1 IS_GAS_PRICE                )     (next IS_GAS_PRICE                ))
        (* (- 1 IS_MAX_PRIORITY_FEE_PER_GAS )     (next IS_MAX_PRIORITY_FEE_PER_GAS ))
        (* (- 1 IS_MAX_FEE_PER_GAS          )     (next IS_MAX_FEE_PER_GAS          ))
        (* (- 1 IS_GAS_LIMIT                )     (next IS_GAS_LIMIT                ))
        (* (- 1 IS_TO                       )     (next IS_TO                       ))
        (* (- 1 IS_VALUE                    )     (next IS_VALUE                    ))
        (* (- 1 IS_DATA                     )     (next IS_DATA                     ))
        (* (- 1 IS_ACCESS_LIST              )     (next IS_ACCESS_LIST              ))
        (* (- 1 IS_BETA                     )     (next IS_BETA                     ))
        (* (- 1 IS_Y                        )     (next IS_Y                        ))
        (* (- 1 IS_R                        )     (next IS_R                        ))
        (* (- 1 IS_S                        )     (next IS_S                        ))
    )))

(defconstraint persp-flag-is-phase-flag () (eq! (persp-flag-sum) (phase-flag-sum)))

(defconstraint every-phase-starts-with-a-transaction-row () (eq! (about-to-enter-new-phase) (next TXN)))

(defconstraint computation-follows-transaction-row () (if-not-zero TXN (will-eq! CMP 1)))


(defproperty outside-of-padding-rows-entering-a-new-phase-coincides-with-exiting-the-current-one 
    (if-not-zero (phase-flag-sum) 
        (eq! (about-to-enter-new-phase) (about-to-exit-current-phase))))

;; This constraint isn't in the spec AFAICT
;; indeed the spec doesn't contain the
;;
;;    upcoming_phase_transition
;;
;; shorthand.
(defproperty prop-outside-of-padding-rows-entering-a-new-phase-coincides-with-exiting-the-current-one 
    (if-not-zero (phase-flag-sum) 
        (eq! (about-to-enter-new-phase) (upcoming-phase-transition))))
