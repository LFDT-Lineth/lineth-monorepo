(module oob)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   OOB_INST_MODEXP_pricing   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF___MODEXP_PRICING___RAC_ISZERO_CHECK                 0
  ROFF___MODEXP_PRICING___EXPONENT_LOG_ISZERO_CHECK        1
  ROFF___MODEXP_PRICING___CEILING_OF_MAX_OVER_8            2
  ROFF___MODEXP_PRICING___MAX_VS_32                        3
  ROFF___MODEXP_PRICING___RAW_COST_VS_MIN_COST             4
  ROFF___MODEXP_PRICING___CALLEE_GAS_VS_PRECOMPILE_COST    5
  )


(defun (prc-modexp-pricing---standard-precondition)                (*   (assumption---fresh-new-stamp) IS_MODEXP_PRICING))
(defun (prc-modexp-pricing---exponent-log)                         [DATA 6])
(defun (prc-modexp-pricing---max-mbs-bbs)                          [DATA 7])
;; ""



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   row i + 0: r@c.isZero() check   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    prc-modexp-pricing---r@c-iszero-check
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-ISZERO   ROFF___MODEXP_PRICING___RAC_ISZERO_CHECK
                                    0
                                    (prc---r@c)
                                    ))

(defun   (prc-modex-pricing---r@c-is-zero)   (shift   OUTGOING_RES_LO   ROFF___MODEXP_PRICING___RAC_ISZERO_CHECK))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;   row i + 1: exponentLog.isZero() check   ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    prc-modexp-pricing---check-exponent-log-is-zero
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-ISZERO   ROFF___MODEXP_PRICING___EXPONENT_LOG_ISZERO_CHECK
                                    0
                                    (prc-modexp-pricing---exponent-log)
                                    ))

(defun   (prc-modexp-pricing---exponent-log-is-zero)  (shift   OUTGOING_RES_LO   ROFF___MODEXP_PRICING___EXPONENT_LOG_ISZERO_CHECK))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;   row i + 2: ⌈ max(mbs, bbs) / 8 ⌉ compututation   ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint    prc-modexp-pricing---computing-ceiling-of-max-mbs-bbs-over-8
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-DIV   ROFF___MODEXP_PRICING___CEILING_OF_MAX_OVER_8
                                 0
                                 (+ (prc-modexp-pricing---max-mbs-bbs) 7)
                                 0
                                 8
                                 ))

(defun   (prc-modexp-pricing---ceiling-of-max-over-8)   (shift   OUTGOING_RES_LO   ROFF___MODEXP_PRICING___CEILING_OF_MAX_OVER_8))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;   row i + 3: comparing max(mbs, bbs) and 32   ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    prc-modexp-pricing---max-mbs-bbs-vs-32
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT   ROFF___MODEXP_PRICING___MAX_VS_32
                                0
                                WORD_SIZE
                                0
                                (prc-modexp-pricing---max-mbs-bbs)
                                ))

(defun   (prc-modexp-pricing---word-cost-dominates)         (shift   OUTGOING_RES_LO   ROFF___MODEXP_PRICING___MAX_VS_32))
(defun   (prc-modexp-pricing---f-of-max)                    (*  (prc-modexp-pricing---ceiling-of-max-over-8)
                                                                (prc-modexp-pricing---ceiling-of-max-over-8)))
(defun   (prc-modexp-pricing---multiplication-complexity)   (if-zero   (force-bin   (prc-modexp-pricing---word-cost-dominates))
                                                                       16
                                                                       (*   2   (prc-modexp-pricing---f-of-max))))
(defun   (prc-modexp-pricing---iteration-count-or-1)        (if-zero   (force-bin   (prc-modexp-pricing---exponent-log-is-zero))
                                                                       (prc-modexp-pricing---exponent-log)
                                                                       1))
(defun   (prc-modexp-pricing---raw-cost)                    (*  (prc-modexp-pricing---multiplication-complexity)
                                                                (prc-modexp-pricing---iteration-count-or-1)))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;   row i + 4: comparing raw_price to 500   ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint    prc-modexp-pricing---compare-raw-cost-against-GAS_CONST_MODEXP-of-EIP-7823
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT    ROFF___MODEXP_PRICING___RAW_COST_VS_MIN_COST
                                 0
                                 (prc-modexp-pricing---raw-cost)
                                 0
                                 GAS_CONST_MODEXP_EIP_7823
                                 ))

(defun    (prc-modexp-pricing---raw-cost-LT-min-cost)    (force-bin    (shift    OUTGOING_RES_LO    ROFF___MODEXP_PRICING___RAW_COST_VS_MIN_COST)))
(defun    (prc-modexp-pricing---precompile-cost)         (if-zero   (prc-modexp-pricing---raw-cost-LT-min-cost)
                                                                    ;; raw_cost_LT_min_cost ≡ faux
                                                                    (prc-modexp-pricing---raw-cost)
                                                                    ;; raw_cost_LT_min_cost ≡ true
                                                                    GAS_CONST_MODEXP_EIP_7823))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;;   row i + 5: comparing callee_gas to precopile_cost   ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint    prc-modexp-pricing---compare-call-gas-against-precompile-cost
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (call-to-LT    ROFF___MODEXP_PRICING___CALLEE_GAS_VS_PRECOMPILE_COST
                                 0
                                 (prc---callee-gas)
                                 0
                                 (prc-modexp-pricing---precompile-cost)
                                 ))

(defun   (prc-modexp-pricing---callee-gas-LT-precompile-cost)    (force-bin   (shift   OUTGOING_RES_LO    ROFF___MODEXP_PRICING___CALLEE_GAS_VS_PRECOMPILE_COST)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;   justifying HUB predictions   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    prc-modexp-pricing---justify-hub-predictions---ram-success
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   (prc---ram-success)
                         (- 1 (prc-modexp-pricing---callee-gas-LT-precompile-cost))
                         ))

(defconstraint    prc-modexp-pricing---justify-hub-predictions---return-gas
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    (force-bin   (prc---ram-success))
                              ;; ram_success ≡ faux
                              (vanishes!   (prc---return-gas))
                              ;; ram_success ≡ true
                              (eq!         (prc---return-gas)
                                           (-   (prc---callee-gas)
                                                (prc-modexp-pricing---precompile-cost)))
                              ))

(defconstraint    prc-modexp-pricing---justify-hub-predictions---r@c-nonzero
                  (:guard  (prc-modexp-pricing---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   (prc---r@c-nonzero)
                         (-  1  (prc-modex-pricing---r@c-is-zero))
                         ))

