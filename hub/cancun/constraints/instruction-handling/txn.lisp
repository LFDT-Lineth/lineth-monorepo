(module hub)

(defun (txn-instruction---standard-precondition) (force-bin (* PEEK_AT_STACK stack/TXN_FLAG (- 1 stack/SUX stack/SOX))))
(defun (txn-instruction---is-ORIGIN)      (force-bin [ stack/DEC_FLAG 1 ]))
(defun (txn-instruction---is-GASPRICE)    (force-bin [ stack/DEC_FLAG 2 ]))
(defun (txn-instruction---is-BLOBHASH)    (force-bin [ stack/DEC_FLAG 3 ]))
(defun (txn-instruction---result-hi)      [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun (txn-instruction---result-lo)      [ stack/STACK_ITEM_VALUE_LO 4 ]) ;; ""

(defconst
  roff---txn-instruction---transaction-row           1
  roff---txn-instruction---exceptional-context-row   2
  )

(defconstraint    txn-instruction---setting-the-stack-pattern
                  (:guard (txn-instruction---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin 
                  (if-eq (txn-instruction---is-ORIGIN)   1 (stack-pattern-0-1))
                  (if-eq (txn-instruction---is-GASPRICE) 1 (stack-pattern-0-1))
                  (if-eq (txn-instruction---is-BLOBHASH) 1 (stack-pattern-1-1))))

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
                                  (if-eq    (txn-instruction---is-ORIGIN) 1
                                                  (begin (eq! (txn-instruction---result-hi) (shift   transaction/FROM_ADDRESS_HI   roff---txn-instruction---transaction-row))
                                                         (eq! (txn-instruction---result-lo) (shift   transaction/FROM_ADDRESS_LO   roff---txn-instruction---transaction-row))))
                                  (if-eq    (txn-instruction---is-GASPRICE) 1
                                                  (begin (eq! (txn-instruction---result-hi) 0)
                                                         (eq! (txn-instruction---result-lo) (shift   transaction/GAS_PRICE         roff---txn-instruction---transaction-row))))
                                  (if-eq    (txn-instruction---is-BLOBHASH) 1
                                                  (begin (eq! (txn-instruction---result-hi) 0)
                                                         (eq! (txn-instruction---result-lo) 0)))))))
