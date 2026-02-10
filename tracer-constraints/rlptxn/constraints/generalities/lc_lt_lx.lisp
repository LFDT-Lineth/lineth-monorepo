(module rlptxn)

(defun (limb-unconditionally-partakes-in-LX)
  (force-bin (+
               ;; IS_RLP_PREFIX
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
               ;; IS_BETA
               ;; IS_Y
               ;; IS_R
               ;; IS_S
               )))

(defun (limb-unconditionally-partakes-in-LT)
(force-bin (+
        (limb-unconditionally-partakes-in-LX)
        IS_Y
        IS_R
        IS_S
    )))

;; ;; I've added them to the :binary@prove columns
;; (defproperty lc-lt-lx-binaries
;;     (begin
;;     (is-binary LC)
;;     (is-binary LX)
;;     (is-binary LT)))

(defconstraint counter-constancies-for-LT-and-LX ()
    (begin
    (counter-constant  LT  CT)
    (counter-constant  LX  CT)
    ))

(defconstraint setting-LT-and-LX-for-most-phases ()
    (if-not-zero (limb-unconditionally-partakes-in-LT)
        (begin
        (eq! LT (limb-unconditionally-partakes-in-LT))
        (eq! LX (limb-unconditionally-partakes-in-LX)))))

(defconstraint LC-may-only-turn-on-along-computation-rows ()
    (if-zero CMP (vanishes! LC)))

(defproperty LT-and-LX-automatically-vanish-on-padding-rows
    (if-zero (phase-flag-sum)
        (begin
        (vanishes! LT)
        (vanishes! LX))))
