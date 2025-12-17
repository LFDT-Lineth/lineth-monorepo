(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3 Computations and checks  ;;
;;  3.X For COINBASE           ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (coinbase-precondition) (* (- 1 (prev IS_CB)) IS_CB))
(defun (curr-COINBASE-hi)      (curr-data-hi))
(defun (curr-COINBASE-lo)      (curr-data-lo))

(defconstraint   coinbase---horizontalization
                 (:guard (coinbase-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin (eq!   (curr-COINBASE-hi)   COINBASE_HI)
                        (eq!   (curr-COINBASE-lo)   COINBASE_LO)))

;; ensures that the coinbase is a 20 = 4 + 16 byte integer
(defconstraint   coinbase---upper-bound
                 (:guard (coinbase-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-LT    0
                                    (curr-COINBASE-hi)
                                    (curr-COINBASE-lo)
                                    (^ 256 4)
                                    0))
