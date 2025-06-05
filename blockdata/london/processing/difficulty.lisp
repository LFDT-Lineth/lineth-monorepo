(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For DIFFICULTY         ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (difficulty-precondition) (* (- 1 (prev IS_DF)) IS_DF))
(defun (curr-DIFFICULTY-hi)      (curr-data-hi))
(defun (curr-DIFFICULTY-lo)      (curr-data-lo))

(defconstraint   difficulty---smallness-proof
                 (:guard (difficulty-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ   0
                                    (curr-DIFFICULTY-hi)
                                    (curr-DIFFICULTY-lo)
                                    0
                                    0))
