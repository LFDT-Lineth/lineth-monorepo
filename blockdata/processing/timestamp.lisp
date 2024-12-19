(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For TIMESTAMP          ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (timestamp-precondition) (* (- 1 (prev IS_TS)) IS_TS))
(defun (curr-TIMESTAMP-hi)      (curr-data-hi))
(defun (curr-TIMESTAMP-lo)      (curr-data-lo))
(defun (prev-TIMESTAMP-hi)      (prev-data-hi))
(defun (prev-TIMESTAMP-lo)      (prev-data-lo))

(defconstraint   timestamp---upper-bound
                 (:guard (timestamp-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-LT    0
                                    (curr-TIMESTAMP-hi)
                                    (curr-TIMESTAMP-lo)
                                    0
                                    (^ 256 6))) ;; ""

(defconstraint   timestamp---is-incrementing
                 (:guard (timestamp-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (isnt-first-block-in-conflation)
                              (wcp-call-to-GT   1
                                                (curr-TIMESTAMP-hi)
                                                (curr-TIMESTAMP-lo)
                                                (prev-TIMESTAMP-hi)
                                                (prev-TIMESTAMP-lo)
                                                )))

