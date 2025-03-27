(module gas)

;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;  3.1 Binarities  ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;
(defconstraint binary-constraints ()
  (if-not-zero OOGX
              (eq! XAHOY 1)))

;; others are done with binary@prove in columns.lisp

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    3.2 Heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; 1
(defconstraint first-row (:domain {0})
  (vanishes! IOMF))

;; 2
(defconstraint iomf-increments ()
  (or! (will-remain-constant! IOMF) (will-inc! IOMF 1)))

;; 3
(defconstraint iomf-vanishing-values ()
  (if-zero IOMF
           (begin (vanishes! FIRST)
                  (debug (vanishes! CT))
                  (vanishes! (next CT))
                  (debug (vanishes! CT_MAX))
                  (debug (vanishes! GAS_ACTUAL))
                  (debug (vanishes! GAS_COST))
                  (debug (vanishes! OOGX))
                  (debug (vanishes! XAHOY)))))

;; 4
(defconstraint instruction-counter-cycle ()
  (if-not-zero IOMF
               (begin (eq! CT_MAX
                           (- 2
                              (* XAHOY (- 1 OOGX))))
                      (if-zero CT
                               (eq! FIRST 1)
                               (eq! FIRST 0))
                      (if-eq-else CT CT_MAX
                                  (vanishes! (next CT))
                                  (will-inc! CT 1)))))

;; 5
(defconstraint final-row (:domain {-1})
  (if-not-zero IOMF
               (eq! CT CT_MAX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3 Constancy constraints  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint counter-constancy ()
  (begin (counter-constancy CT GAS_ACTUAL)
         (counter-constancy CT GAS_COST)
         (counter-constancy CT XAHOY)
         (counter-constancy CT OOGX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;  3.4 Populating the lookup columns  ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; defining "WCP macros"
(defun (call-to-LT k  ;; row shift parameter
                   a  ;; arg 1 low
                   b  ;; arg 2 low
                   c) ;; res
  (begin (eq! (shift WCP_ARG1_LO k) a)
         (eq! (shift WCP_ARG2_LO k) b)
         (eq! (shift WCP_INST k) EVM_INST_LT)
         (eq! (shift WCP_RES k) c)))

(defun (call-to-LEQ k  ;; row shift parameter
                    a  ;; arg 1 low
                    b  ;; arg 2 low
                    c) ;; res
  (begin (eq! (shift WCP_ARG1_LO k) a)
         (eq! (shift WCP_ARG2_LO k) b)
         (eq! (shift WCP_INST k) WCP_INST_LEQ)
         (eq! (shift WCP_RES k) c)))

(defconstraint asserting-the-leftover-gas-is-nonnegative (:guard FIRST)
  (call-to-LEQ 0          ;; row shift parameter
               0          ;; arg 1 low
               GAS_ACTUAL ;; arg 2 low
               1))        ;; res is TRUE!

;; as per the spec, this constraint the following
;; constraint is slightly useless ... not entirely,
;; though: it still asserts "smallness" so that it
;; should filter out MXPX induced out of gas exceptions.
(defconstraint asserting-the-gas-cost-is-nonnegative (:guard FIRST)
  (call-to-LEQ 1       ;; row shift parameter
              0        ;; arg 1 low
              GAS_COST ;; arg 2 low
              1))      ;; res is TRUE!

(defconstraint asserting-either-sufficient-gas-or-insufficient-gas (:guard FIRST)
  (if-zero (force-bin (* XAHOY (- 1 OOGX)))
           (call-to-LT 2          ;; row shift parameter
                       GAS_ACTUAL ;; arg 1 low
                       GAS_COST   ;; arg 2 low
                       OOGX)))    ;; res predicted by HUB


