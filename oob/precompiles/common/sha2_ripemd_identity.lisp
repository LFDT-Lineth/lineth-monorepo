(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;   For SHA2-256, RIPEMD-160, IDENTITY   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-sha2-prc-ripemd-prc-identity---standard-precondition)     (+ IS_SHA2 IS_RIPEMD IS_IDENTITY))
(defun (prc-sha2-prc-ripemd-prc-identity---ceil)                      (shift OUTGOING_RES_LO 2))
(defun (prc-sha2-prc-ripemd-prc-identity---insufficient-gas)          (shift OUTGOING_RES_LO 3))
(defun (prc-sha2-prc-ripemd-prc-identity---sha2-cost)                 (+  GAS_CONST_SHA2       (* GAS_CONST_SHA2_WORD      (prc-sha2-prc-ripemd-prc-identity---ceil))))
(defun (prc-sha2-prc-ripemd-prc-identity---ripemd-cost)               (+  GAS_CONST_RIPEMD     (* GAS_CONST_RIPEMD_WORD    (prc-sha2-prc-ripemd-prc-identity---ceil))))
(defun (prc-sha2-prc-ripemd-prc-identity---identity-cost)             (+  GAS_CONST_IDENTITY   (* GAS_CONST_IDENTITY_WORD  (prc-sha2-prc-ripemd-prc-identity---ceil))))
(defun (prc-sha2-prc-ripemd-prc-identity---precompile-cost)           (+  (*  (prc-sha2-prc-ripemd-prc-identity---sha2-cost)      IS_SHA2    )
                                                                          (*  (prc-sha2-prc-ripemd-prc-identity---ripemd-cost)    IS_RIPEMD  )
                                                                          (*  (prc-sha2-prc-ripemd-prc-identity---identity-cost)  IS_IDENTITY)))

(defconstraint prc-sha2-prc-ripemd-prc-identity---div-cds-plus-31-by-32 (:guard (* (assumption---fresh-new-stamp) (prc-sha2-prc-ripemd-prc-identity---standard-precondition)))
  (call-to-DIV 2 0 (+ (prc---cds) 31) 0 32))

(defconstraint prc-sha2-prc-ripemd-prc-identity---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-sha2-prc-ripemd-prc-identity---standard-precondition)))
  (call-to-LT 3 0 (prc---callee-gas) 0 (prc-sha2-prc-ripemd-prc-identity---precompile-cost)))

(defconstraint prc-sha2-prc-ripemd-prc-identity---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-sha2-prc-ripemd-prc-identity---standard-precondition)))
  (begin (eq! (prc---hub-success) (- 1 (prc-sha2-prc-ripemd-prc-identity---insufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---callee-gas) (prc-sha2-prc-ripemd-prc-identity---precompile-cost))))))
