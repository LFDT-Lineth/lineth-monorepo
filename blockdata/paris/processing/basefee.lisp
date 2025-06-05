(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For BASEFEE            ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (basefee-precondition) (* (- 1 (prev IS_BF)) IS_BF))
(defun (curr-BASEFEE-hi)      (curr-data-hi))
(defun (curr-BASEFEE-lo)      (curr-data-lo))

(defconstraint   basefee---horizontalization
                 (:guard (basefee-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin (eq!  (curr-BASEFEE-hi)  0)
                        (eq!  (curr-BASEFEE-lo)  BASEFEE)))

(defconstraint   basefee---size-constraint
                 (:guard (basefee-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ   0
                                    (curr-BASEFEE-hi)
                                    (curr-BASEFEE-lo)
                                    0
                                    0))
