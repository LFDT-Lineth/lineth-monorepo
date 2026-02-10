(module rlptxn)

(defun (phase-flag-sum)
    (force-bin
      (+ IS_RLP_PREFIX
         IS_CHAIN_ID
         IS_NONCE
         IS_GAS_PRICE
         IS_MAX_PRIORITY_FEE_PER_GAS
         IS_MAX_FEE_PER_GAS
         IS_GAS_LIMIT
         IS_TO
         IS_VALUE
         IS_DATA
         IS_ACCESS_LIST
         IS_AUTHORIZATION_LIST
         IS_BETA
         IS_Y
         IS_R
         IS_S
         )))

(defconstraint the-phase-flag-vanishes-precisely-along-padding-rows ()
    (if-zero USER_TXN_NUMBER
        (vanishes! (phase-flag-sum))
        (eq! (phase-flag-sum) 1)))
