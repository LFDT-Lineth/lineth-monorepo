(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For PREVRANDAO         ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prevrandao-precondition) (* (- 1 (prev IS_PR)) IS_PR))
(defun (curr-PREVRANDAO-hi)      (curr-data-hi))
(defun (curr-PREVRANDAO-lo)      (curr-data-lo))

(defconstraint   prevrandao---smallness-proof
                 (:guard (prevrandao-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ   0
                                    (curr-PREVRANDAO-hi)
                                    (curr-PREVRANDAO-lo)
                                    0
                                    0))
