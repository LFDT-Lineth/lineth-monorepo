(module oob)


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;   OOB_INST_CDL   ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;

;; Note. We use cdl as a shorthand for CALLDATALOAD

(defun (cdl---standard-precondition)    IS_CDL)
(defun (cdl---offset-hi)                [DATA 1])
(defun (cdl---offset-lo)                [DATA 2])
(defun (cdl---cds)                      [DATA 5])
(defun (cdl---cdl-out-of-bounds)        [DATA 7])
(defun (cdl---touches-ram)              OUTGOING_RES_LO)

(defconstraint cdl---compare-offset-against-cds (:guard (* (assumption---fresh-new-stamp) (cdl---standard-precondition)))
  (call-to-LT 0 (cdl---offset-hi) (cdl---offset-lo) 0 (cdl---cds)))

(defconstraint cdl---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (cdl---standard-precondition)))
  (eq! (cdl---cdl-out-of-bounds) (- 1 (cdl---touches-ram))))
