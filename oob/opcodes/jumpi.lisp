(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;   OOB_INST_JUMPI   ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defun (jumpi---standard-precondition)      IS_JUMPI)
(defun (jumpi---pc-new-hi)                  [DATA 1])
(defun (jumpi---pc-new-lo)                  [DATA 2])
(defun (jumpi---jump-cond-hi)               [DATA 3])
(defun (jumpi---jump-cond-lo)               [DATA 4])
(defun (jumpi---code-size)                  [DATA 5])
(defun (jumpi---jump-not-attempted)         [DATA 6])
(defun (jumpi---guaranteed-exception)       [DATA 7])
(defun (jumpi---jump-must-be-attempted)     [DATA 8])
(defun (jumpi---valid-pc-new)               OUTGOING_RES_LO)
(defun (jumpi---jump-cond-is-zero)          (next OUTGOING_RES_LO))

(defconstraint jumpi---compare-pc-new-against-code-size (:guard (* (assumption---fresh-new-stamp) (jumpi---standard-precondition)))
  (call-to-LT 0 (jumpi---pc-new-hi) (jumpi---pc-new-lo) 0 (jumpi---code-size)))

(defconstraint jumpi---check-jump-cond-is-zero (:guard (* (assumption---fresh-new-stamp) (jumpi---standard-precondition)))
  (call-to-ISZERO 1 (jumpi---jump-cond-hi) (jumpi---jump-cond-lo)))

(defconstraint jumpi---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (jumpi---standard-precondition)))
  (begin (eq! (jumpi---jump-not-attempted) (jumpi---jump-cond-is-zero))
         (eq! (jumpi---guaranteed-exception)
              (* (- 1 (jumpi---jump-cond-is-zero)) (- 1 (jumpi---valid-pc-new))))
         (eq! (jumpi---jump-must-be-attempted)
              (* (- 1 (jumpi---jump-cond-is-zero)) (jumpi---valid-pc-new)))
         (debug (is-binary (jumpi---jump-not-attempted)))
         (debug (is-binary (jumpi---guaranteed-exception)))
         (debug (is-binary (jumpi---jump-must-be-attempted)))
         (debug (eq! (+ (jumpi---guaranteed-exception)
                        (jumpi---jump-must-be-attempted)
                        (jumpi---jump-not-attempted))
                     1))))
