(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   OOB_INST_MODEXP_pricing   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-pricing---standard-precondition)                IS_MODEXP_PRICING)
(defun (prc-modexp-pricing---exponent-log)                         [DATA 6])
(defun (prc-modexp-pricing---max-xbs-ybs)                          [DATA 7])
(defun (prc-modexp-pricing---exponent-log-is-zero)                 (next OUTGOING_RES_LO))
(defun (prc-modexp-pricing---f-of-max)                             (*  (shift OUTGOING_RES_LO 2)  (shift OUTGOING_RES_LO 2)))
(defun (prc-modexp-pricing---big-quotient)                         (shift OUTGOING_RES_LO 3))
(defun (prc-modexp-pricing---big-quotient_LT_GAS_CONST_MODEXP)     (shift OUTGOING_RES_LO 4))
(defun (prc-modexp-pricing---big-numerator)                        (if-zero (prc-modexp-pricing---exponent-log-is-zero)
                                                                            (* (prc-modexp-pricing---f-of-max) (prc-modexp-pricing---exponent-log))
                                                                            (prc-modexp-pricing---f-of-max)))
(defun (prc-modexp-pricing---precompile-cost)                      (if-zero (prc-modexp-pricing---big-quotient_LT_GAS_CONST_MODEXP)
                                                                            (prc-modexp-pricing---big-quotient)
                                                                            GAS_CONST_MODEXP))

(defconstraint prc-modexp-pricing---check--is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-ISZERO 0 0 (prc---r@c)))

(defconstraint prc-modexp-pricing---check-exponent-log-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-ISZERO 1 0 (prc-modexp-pricing---exponent-log)))

(defconstraint prc-modexp-pricing---div-max-xbs-ybs-plus-7-by-8 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-DIV 2
               0
               (+ (prc-modexp-pricing---max-xbs-ybs) 7)
               0
               8))

(defconstraint prc-modexp-pricing---div-big-numerator-by-quaddivisor (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-DIV 3 0 (prc-modexp-pricing---big-numerator) 0 G_QUADDIVISOR))

(defconstraint prc-modexp-pricing---compare-big-quotient-against-GAS_CONST_MODEXP (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-LT 4 0 (prc-modexp-pricing---big-quotient) 0 GAS_CONST_MODEXP))

(defconstraint prc-modexp-pricing---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-LT 5 0 (prc---callee-gas) 0 (prc-modexp-pricing---precompile-cost)))

(defconstraint prc-modexp-pricing---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (begin (eq! (prc---ram-success)
              (- 1 (shift OUTGOING_RES_LO 5)))
         (if-zero (prc---ram-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas) (- (prc---callee-gas) (prc-modexp-pricing---precompile-cost))))
         (eq! (prc---r@c-nonzero) (- 1 OUTGOING_RES_LO))))

