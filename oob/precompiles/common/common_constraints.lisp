(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   OOB_INST_prc_common   ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-common---standard-precondition)   (flag-sum-prc-common))
(defun (prc---callee-gas)                     [DATA 1])
(defun (prc---cds)                            [DATA 2])
(defun (prc---r@c)                            [DATA 3])
(defun (prc---hub-success)                    [DATA 4])
(defun (prc---ram-success)                    [DATA 4])
(defun (prc---return-gas)                     [DATA 5])
(defun (prc---extract-call-data)              [DATA 6])
(defun (prc---empty-call-data)                [DATA 7])
(defun (prc---r@c-nonzero)                    [DATA 8])
(defun (prc---cds-is-zero)                    OUTGOING_RES_LO)
(defun (prc---r@c-is-zero)                    (next OUTGOING_RES_LO))

(defconstraint prc---check-cds-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-common---standard-precondition)))
  (call-to-ISZERO 0 0 (prc---cds)))

(defconstraint prc---check-r@c-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-common---standard-precondition)))
  (call-to-ISZERO 1 0 (prc---r@c)))

(defconstraint prc---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-common---standard-precondition)))
  (begin (eq! (prc---extract-call-data)
              (* (prc---hub-success) (- 1 (prc---cds-is-zero))))
         (eq! (prc---empty-call-data) (* (prc---hub-success) (prc---cds-is-zero)))
         (eq! (prc---r@c-nonzero) (- 1 (prc---r@c-is-zero)))))
