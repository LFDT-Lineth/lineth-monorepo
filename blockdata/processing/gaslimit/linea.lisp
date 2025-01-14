(module blockdata)

;; TODO add reference to global constants
(defconst GAS_LIMIT_MINIMUM     61000000)
(defconst GAS_LIMIT_MAXIMUM   2000000000)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;  3 Computations and checks   ;;
;;  3.X For GASLIMIT for Linea  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   gaslimit---lower-bound---LINEA
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ   0
                                    (curr-GASLIMIT-hi)
                                    (curr-GASLIMIT-lo)
                                    0
                                    GAS_LIMIT_MINIMUM))

(defconstraint   gaslimit---upper-bound---LINEA
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-LEQ   1 
                                    (curr-GASLIMIT-hi)
                                    (curr-GASLIMIT-lo)
                                    0
                                    GAS_LIMIT_MAXIMUM))
