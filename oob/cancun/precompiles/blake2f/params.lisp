(module oob)


;;;;;;;;;;;;;;;;;;;::;;
;;                   ;;
;;   BLAKE2F_params  ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-blake-params---standard-precondition)          IS_BLAKE2F_PARAMS)
(defun (prc-blake-params---blake-r)                        [DATA 6])
(defun (prc-blake-params---blake-f)                        [DATA 7])
(defun (prc-blake-params---sufficient-gas)                 (- 1 OUTGOING_RES_LO))
(defun (prc-blake-params---f-is-a-bit)                     (next OUTGOING_RES_LO))


(defconstraint prc-blake-params---compare-call-gas-against-blake-r (:guard (* (assumption---fresh-new-stamp) (prc-blake-params---standard-precondition)))
  (call-to-LT 0 0 (prc---callee-gas) 0 (* GAS_CONST_BLAKE2_PER_ROUND (prc-blake-params---blake-r))))

(defconstraint prc-blake-params---compare-blake-f-against-blake-f-square (:guard (* (assumption---fresh-new-stamp) (prc-blake-params---standard-precondition)))
  (call-to-EQ 1
              0
              (prc-blake-params---blake-f)
              0
              (* (prc-blake-params---blake-f) (prc-blake-params---blake-f))))

(defconstraint prc-blake-params---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-blake-params---standard-precondition)))
  (begin (eq! (prc---ram-success)
              (* (prc-blake-params---sufficient-gas) (prc-blake-params---f-is-a-bit)))
         (if-not-zero (prc---ram-success)
                      (eq! (prc---return-gas) (- (prc---callee-gas) (prc-blake-params---blake-r)))
                      (vanishes! (prc---return-gas)))))
