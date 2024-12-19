(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;  3 Computations and checks      ;;
;;  3.X For GASLIMIT for Ethereum  ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   gaslimit---lower-bound---ETHEREUM
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-GEQ   0
                                    (curr-GASLIMIT-hi)
                                    (curr-GASLIMIT-lo)
                                    0
                                    ETHEREUM_GAS_LIMIT_MINIMUM))

(defconstraint   gaslimit---upper-bound---ETHEREUM
                 (:guard (gaslimit-precondition))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (wcp-call-to-LEQ   1 
                                    (curr-GASLIMIT-hi)
                                    (curr-GASLIMIT-lo)
                                    0
                                    ETHEREUM_GAS_LIMIT_MAXIMUM))
