(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For CHAINID            ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (chainid-precondition) (* (- 1 (prev IS_ID)) IS_ID))

(defconstraint   chainid-permanence
                 (:guard (chainid-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (isnt-first-block-in-conflation)
                                (begin (eq! DATA_HI (shift DATA_HI (* nROWS_DEPTH -1)))
                                       (eq! DATA_LO (shift DATA_LO (* nROWS_DEPTH -1))))))

(defconstraint   chainid-bound
                 (:guard (chainid-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ 0 DATA_HI DATA_LO 0 0))
