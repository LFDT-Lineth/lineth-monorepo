(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   For ECRECOVER, ECADD, ECMUL    ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-ecrecover-prc-ecadd-prc-ecmul---standard-precondition)    (+ IS_ECRECOVER IS_ECADD IS_ECMUL))
(defun (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost)          (+ (* GAS_CONST_ECRECOVER IS_ECRECOVER) (* GAS_CONST_ECADD IS_ECADD) (* GAS_CONST_ECMUL IS_ECMUL)))
(defun (prc-ecrecover-prc-ecadd-prc-ecmul---insufficient-gas)         (shift OUTGOING_RES_LO 2))

(defconstraint prc-ecrecover-prc-ecadd-prc-ecmul---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-ecrecover-prc-ecadd-prc-ecmul---standard-precondition)))
  (call-to-LT 2 0 (prc---callee-gas) 0 (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost)))

(defconstraint prc-ecrecover-prc-ecadd-prc-ecmul---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-ecrecover-prc-ecadd-prc-ecmul---standard-precondition)))
  (begin (eq! (prc---hub-success) (- 1 (prc-ecrecover-prc-ecadd-prc-ecmul---insufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---callee-gas) (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost))))))
