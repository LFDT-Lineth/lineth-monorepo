(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For GASLIMIT           ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (gaslimit-precondition) (* (- 1 (prev IS_GL)) IS_GL))
(defun (prev-gas-limit)        (shift BLOCK_GAS_LIMIT (* nROWS_DEPTH -1)))
(defun (curr-GASLIMIT-hi)      (curr-data-hi))
(defun (curr-GASLIMIT-lo)      (curr-data-lo))
(defun (prev-GASLIMIT-hi)      (prev-data-hi))
(defun (prev-GASLIMIT-lo)      (prev-data-lo))

(defconstraint   gaslimit---horizontalization
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin (eq!  (curr-GASLIMIT-hi)  0)
                        (eq!  (curr-GASLIMIT-lo)  BLOCK_GAS_LIMIT)))

;; row i + 0
;; row i + 1
;; are ethereum vs. linea dependent, see ethereum.lisp and linea.lisp

(defconstraint   gaslimit---compute-maximum-deviation
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (isnt-first-block-in-conflation)
                                (euc-call   2
                                            (prev-gas-limit)
                                            GAS_LIMIT_ADJUSTMENT_FACTOR)))

(defun (max-deviation)         (shift RES 2))

(defconstraint   gaslimit---compare-to-upper-bound
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (isnt-first-block-in-conflation)
                                (wcp-call-to-LT   3
                                                  (curr-GASLIMIT-hi)
                                                  (curr-GASLIMIT-lo)
                                                  0
                                                  (+ (prev-gas-limit) (max-deviation)))))

(defconstraint   gaslimit---compare-to-lower-bound
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (isnt-first-block-in-conflation)
                                (wcp-call-to-GT   4 
                                                  (curr-GASLIMIT-hi)
                                                  (curr-GASLIMIT-lo)
                                                  0
                                                  (- (prev-gas-limit) (max-deviation)))))

