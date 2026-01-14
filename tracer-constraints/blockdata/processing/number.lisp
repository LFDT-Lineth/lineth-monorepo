(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For NUMBER             ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (number-precondition) (* (- 1 (prev IS_NB)) IS_NB))
(defun (curr-NUMBER-hi)      (curr-data-hi))
(defun (curr-NUMBER-lo)      (curr-data-lo))
(defun (prev-NUMBER-hi)      (prev-data-hi))
(defun (prev-NUMBER-lo)      (prev-data-lo))

(defconstraint   number---checking-whether-the-first-block-in-the-conflation-is-the-genesis-block
                 (:guard (number-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-ISZERO   0
                                       0
                                       FIRST_BLOCK_NUMBER))

(defun (first-block-is-genesis-block) RES)

(defconstraint   number---upper-bound
                 (:guard (number-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (is-first-block-in-conflation)
                                (wcp-call-to-LT   1
                                                  (curr-NUMBER-hi)
                                                  (curr-NUMBER-lo)
                                                  0
                                                  (^ 256 8)
                                                  ))) ;; ""

(defconstraint   number---connecting-constraints
                 (:guard (number-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   (is-first-block-in-conflation)
                                  (begin (vanishes!  (curr-NUMBER-hi))
                                         (eq!        (curr-NUMBER-lo)    FIRST_BLOCK_NUMBER)))
                   (if-not-zero   (isnt-first-block-in-conflation)
                                  (begin (eq!  (curr-NUMBER-hi)      (prev-NUMBER-hi))
                                         (eq!  (curr-NUMBER-lo)  (+  (prev-NUMBER-lo)  1))))
                   ))

(defconstraint   number---setting-the-eponymous-column
                 (:guard (number-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!    NUMBER    (curr-NUMBER-lo))
                   (eq!    NUMBER    (+    FIRST_BLOCK_NUMBER
                                           (-    REL_BLOCK
                                                 1)))
                   ))
