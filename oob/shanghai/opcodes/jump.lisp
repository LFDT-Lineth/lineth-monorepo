(module oob)


;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;   OOB_INST_JUMP   ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun (jump---standard-precondition)       IS_JUMP)
(defun (jump---pc-new-hi)                   [DATA 1])
(defun (jump---pc-new-lo)                   [DATA 2])
(defun (jump---code-size)                   [DATA 5])
(defun (jump---guaranteed-exception)        [DATA 7])
(defun (jump---jump-must-be-attempted)      [DATA 8])
(defun (jump---valid-pc-new)                OUTGOING_RES_LO)

(defconstraint jump---compare-pc-new-against-code-size (:guard (* (assumption---fresh-new-stamp) (jump---standard-precondition)))
  (call-to-LT 0 (jump---pc-new-hi) (jump---pc-new-lo) 0 (jump---code-size)))

(defconstraint jump---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (jump---standard-precondition)))
  (begin (eq! (jump---guaranteed-exception) (- 1 (jump---valid-pc-new)))
         (eq! (jump---jump-must-be-attempted) (jump---valid-pc-new))
         (debug (is-binary (jump---guaranteed-exception)))
         (debug (is-binary (jump---jump-must-be-attempted)))
         (debug (eq! (+ (jump---guaranteed-exception) (jump---jump-must-be-attempted)) 1))))
