(module hub)

(defun (txn-instruction---standard-precondition) (* PEEK_AT_STACK stack/TXN_FLAG (- 1 stack/SUX stack/SOX)))
(defun (txn-instruction---is-ORIGIN)      [ stack/DEC_FLAG 1 ])
(defun (txn-instruction---is-GASPRICE)    [ stack/DEC_FLAG 2 ])
(defun (txn-instruction---result-hi)      [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun (txn-instruction---result-lo)      [ stack/STACK_ITEM_VALUE_LO 4 ]) ;; ""

(defconst
  roff---txn-instruction---transaction-row           1
  roff---txn-instruction---exceptional-context-row   2
  )

(defconstraint    txn-instruction---setting-the-stack-pattern
                  (:guard (txn-instruction---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (stack-pattern-0-1))

(defconstraint    txn-instruction---setting-NSR
                  (:guard (txn-instruction---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq! NSR (+ 1 CMC)))

(defconstraint    txn-instruction---setting-the-peeking-flags
                  (:guard (txn-instruction---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq! NSR
                       (+ (shift     PEEK_AT_TRANSACTION  roff---txn-instruction---transaction-row)
                          (* (shift  PEEK_AT_CONTEXT      roff---txn-instruction---exceptional-context-row) CMC))))

(defconstraint    txn-instruction---setting-the-gas-cost
                  (:guard (txn-instruction---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq! GAS_COST stack/STATIC_GAS))

(defconstraint    txn-instruction---setting-the-result
                  (:guard (txn-instruction---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-zero    XAHOY
                                (begin
                                  (if-not-zero    (txn-instruction---is-ORIGIN)
                                                  (begin (eq! (txn-instruction---result-hi) (shift   transaction/FROM_ADDRESS_HI   roff---txn-instruction---transaction-row))
                                                         (eq! (txn-instruction---result-lo) (shift   transaction/FROM_ADDRESS_LO   roff---txn-instruction---transaction-row))))
                                  (if-not-zero    (txn-instruction---is-GASPRICE)
                                                  (begin (eq! (txn-instruction---result-hi) 0)
                                                         (eq! (txn-instruction---result-lo) (shift   transaction/GAS_PRICE         roff---txn-instruction---transaction-row))))))))
