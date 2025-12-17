(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;   OOB_INST_XCALL   ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

;; Note. We use XCALL as a shorthand for "eXceptional CALL-type instruction"

(defun (xcall---standard-precondition)      IS_XCALL)
(defun (xcall---value-hi)                   [DATA 1])
(defun (xcall---value-lo)                   [DATA 2])
(defun (xcall---value-is-nonzero)           [DATA 7])
(defun (xcall---value-is-zero)              [DATA 8])

(defconstraint xcall---check-value-is-zero (:guard (* (assumption---fresh-new-stamp) (xcall---standard-precondition)))
  (call-to-ISZERO 0 (xcall---value-hi) (xcall---value-lo)))

(defconstraint xcall---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (xcall---standard-precondition)))
  (begin (eq! (xcall---value-is-nonzero) (- 1 OUTGOING_RES_LO))
         (eq! (xcall---value-is-zero) OUTGOING_RES_LO)))
