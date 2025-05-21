(module oob)


;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;   OOB_INST_CALL   ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun (call---standard-precondition)           IS_CALL)
(defun (call---value-hi)                        [DATA 1])
(defun (call---value-lo)                        [DATA 2])
(defun (call---balance)                         [DATA 3])
(defun (call---call-stack-depth)                [DATA 6])
(defun (call---value-is-nonzero)                [DATA 7])
(defun (call---aborting-condition)              [DATA 8])
(defun (call---insufficient-balance-abort)      OUTGOING_RES_LO)
(defun (call---call-stack-depth-abort)          (- 1 (next OUTGOING_RES_LO)))
(defun (call---value-is-zero)                   (shift OUTGOING_RES_LO 2))

(defconstraint call---compare-balance-against-value (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (call-to-LT 0 0 (call---balance) (call---value-hi) (call---value-lo)))

(defconstraint call---compare-call-stack-depth-against-1024 (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (call-to-LT 1 0 (call---call-stack-depth) 0 1024))

(defconstraint call---check-value-is-zero (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (call-to-ISZERO 2 (call---value-hi) (call---value-lo)))

(defconstraint call---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (begin (eq! (call---value-is-nonzero) (- 1 (call---value-is-zero)))
         (eq! (call---aborting-condition)
              (+ (call---insufficient-balance-abort)
                 (* (- 1 (call---insufficient-balance-abort)) (call---call-stack-depth-abort))))))

