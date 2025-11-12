(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For BLOBBASEFEE        ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (blobbasefee-precondition) (* (- 1 (prev IS_BLOBBASEFEE)) IS_BLOBBASEFEE))
(defun (curr-BLOBBASEFEE-hi)      (curr-data-hi))
(defun (curr-BLOBBASEFEE-lo)      (curr-data-lo))

(defconstraint   blobbasefee-bound
                 (:guard (blobbasefee-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ 0 DATA_HI DATA_LO 0 0))
