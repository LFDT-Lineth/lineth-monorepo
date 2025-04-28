(module txndata)

;; sum of transaction type flags
(defun (tx-type-sum) (force-bin (+ TYPE0
                                   TYPE1
                                   TYPE2)))

;; constraint imposing that STAMP[i + 1] ∈ { STAMP[i], 1 + STAMP[i] }
(defpurefun (stamp-progression STAMP)
            (or! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 Heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   first-row (:domain {0}) ;; ""
                 (vanishes! ABS))

(defconstraint   padding-is-padding ()
                 (if-zero ABS
                          (begin (debug (vanishes! CT))
                                 (vanishes! (weighted-sum-of-binary-columns-for-transaction-constancy)))))

(defconstraint   abs-tx-num-increments ()
                 (stamp-progression ABS))

(defconstraint   new-stamp-reboot-ct ()
                 (if-not (will-remain-constant! ABS)
                         (vanishes! (next CT))))

(defconstraint   transactions-have-a-single-type (:guard ABS) (eq! (tx-type-sum) 1))

(defconstraint   counter-column-updates-type-0   (:guard TYPE0)
                 (if-eq-else    CT    (+ CT_MAX_TYPE_0 IS_LAST_TX_OF_BLOCK)
                                (will-inc!   ABS   1)
                                (will-inc!   CT    1)))

(defconstraint   counter-column-updates-type-1   (:guard TYPE1)
                 (if-eq-else    CT    (+ CT_MAX_TYPE_1 IS_LAST_TX_OF_BLOCK)
                                (will-inc!   ABS   1)
                                (will-inc!   CT    1)))

(defconstraint   counter-column-updates-type-2   (:guard TYPE2)
                 (if-eq-else    CT    (+ CT_MAX_TYPE_2 IS_LAST_TX_OF_BLOCK)
                                (will-inc!   ABS   1)
                                (will-inc!   CT    1)))

(defconstraint   final-row (:domain {-1}) ;; ""
                 (begin
                   (eq! ABS ABS_MAX)
                   (eq! REL REL_MAX)
                   (debug   (eq!   IS_LAST_TX_OF_BLOCK   1))
                   (if-not-zero   TYPE0   (eq!   CT   (+   CT_MAX_TYPE_0   1)))
                   (if-not-zero   TYPE1   (eq!   CT   (+   CT_MAX_TYPE_1   1)))
                   (if-not-zero   TYPE2   (eq!   CT   (+   CT_MAX_TYPE_2   1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.2 Constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (transaction-constant X)
  (if-not-zero CT
               (eq! X (prev X))))

(defun (weighted-sum-of-binary-columns-for-transaction-constancy)
  (+ (* (^ 2 0) IS_DEP)
     (* (^ 2 1) TYPE0)
     (* (^ 2 2) TYPE1)
     (* (^ 2 3) TYPE2)
     (* (^ 2 4) REQUIRES_EVM_EXECUTION)
     (* (^ 2 5) COPY_TXCD)
     (* (^ 2 6) STATUS_CODE))) ;; ""

(defconstraint   constancies ()
                 (begin
                   (transaction-constant FROM_HI)
                   (transaction-constant FROM_LO)
                   (transaction-constant NONCE)
                   (transaction-constant VALUE)
                   (transaction-constant GLIM)
                   (transaction-constant TO_HI)
                   (transaction-constant TO_LO)
                   (transaction-constant CALL_DATA_SIZE)
                   (transaction-constant INIT_CODE_SIZE)
                   (transaction-constant IGAS)
                   (transaction-constant PRIORITY_FEE_PER_GAS)
                   (transaction-constant GAS_PRICE)
                   (transaction-constant BASEFEE)
                   (transaction-constant COINBASE_HI)
                   (transaction-constant COINBASE_LO)
                   (transaction-constant CUM_GAS)
                   (transaction-constant CFI)
                   (transaction-constant GAS_LEFTOVER)
                   (transaction-constant REF_CNT)
                   (transaction-constant REFUND_EFFECTIVE)
                   (transaction-constant IBAL)
                   (transaction-constant BLK)
                   (transaction-constant REL)
                   (transaction-constant (weighted-sum-of-binary-columns-for-transaction-constancy))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                        ;;
;;    2.3 Constructing the ABSOLUTE_TRANSACTION_NUMBER    ;;
;;                                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   total-number-constancies ()
                 (begin (if-not-zero ABS
                                     (will-remain-constant! ABS_MAX)
                                     ;; (begin (vanishes! ABS_MAX)
                                     ;;        (vanishes! BLK_MAX)
                                     ;;        (vanishes! REL_MAX)
                                     ;;        (vanishes! BLK)
                                     ;;        (vanishes! REL))
                                     )
                        (if-not (will-inc! BLK 1)
                                (will-remain-constant! REL_MAX))))

(defconstraint   block-number-increments ()
                 (stamp-progression BLK))

(defconstraint   block-and-transaction-number-generalities ()
                 (begin (if-zero ABS
                                 (begin (vanishes! BLK)
                                        (vanishes! REL)
                                        ;;(debug (vanishes! BLK_MAX))
                                        (debug (vanishes! REL_MAX))
                                        (if-not (will-remain-constant! ABS)
                                                (begin (eq! (next BLK) 1)
                                                       (eq! (next REL) 1))))
                                 (if-not (will-remain-constant! ABS)
                                         (if-not-eq REL_MAX
                                                    REL
                                                    (begin (will-remain-constant! BLK)
                                                           (will-inc! REL 1))
                                                    (begin (will-inc! BLK 1)
                                                           (will-eq! REL 1)))))))

(defconstraint   set-last-tx-of-block-flag (:guard ABS_TX_NUM)
                 (if-eq-else REL_TX_NUM REL_TX_NUM_MAX
                             (eq! IS_LAST_TX_OF_BLOCK 1)
                             (vanishes! IS_LAST_TX_OF_BLOCK)))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;    2.6 Aliases    ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun   (tx_type)               (shift OUTGOING_LO 0))
(defun   (optional_to_addr_hi)   (shift OUTGOING_HI 1))
(defun   (optional_to_addr_lo)   (shift OUTGOING_LO 1))
(defun   (nonce)                 (shift OUTGOING_LO 2))
(defun   (is_dep)                (shift OUTGOING_HI 3))
(defun   (value)                 (shift OUTGOING_LO 3))
(defun   (data_cost)             (shift OUTGOING_HI 4))
(defun   (data_size)             (shift OUTGOING_LO 4))
(defun   (gas_limit)             (shift OUTGOING_LO 5))
(defun   (gas_price)             (shift OUTGOING_LO 6))
(defun   (max_priority_fee)      (shift OUTGOING_HI 6))
(defun   (max_fee)               (shift OUTGOING_LO 6))
(defun   (num_keys)              (shift OUTGOING_HI 7))
(defun   (num_addr)              (shift OUTGOING_LO 7))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    2.8 Verticalization    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (setting_phase_numbers)
  (begin (eq! (shift PHASE_RLP_TXN 0) COMMON_RLP_TXN_PHASE_NUMBER_0)
         (eq! (shift PHASE_RLP_TXN 1) COMMON_RLP_TXN_PHASE_NUMBER_1)
         (eq! (shift PHASE_RLP_TXN 2) COMMON_RLP_TXN_PHASE_NUMBER_2)
         (eq! (shift PHASE_RLP_TXN 3) COMMON_RLP_TXN_PHASE_NUMBER_3)
         (eq! (shift PHASE_RLP_TXN 4) COMMON_RLP_TXN_PHASE_NUMBER_4)
         (eq! (shift PHASE_RLP_TXN 5) COMMON_RLP_TXN_PHASE_NUMBER_5)
         ;;
         (if-not-zero TYPE0 (eq! (shift PHASE_RLP_TXN 6) TYPE_0_RLP_TXN_PHASE_NUMBER_6))
         ;;
         (if-not-zero TYPE1 (eq! (shift PHASE_RLP_TXN 6) TYPE_1_RLP_TXN_PHASE_NUMBER_6))
         (if-not-zero TYPE1 (eq! (shift PHASE_RLP_TXN 7) TYPE_1_RLP_TXN_PHASE_NUMBER_7))
         ;;
         (if-not-zero TYPE2 (eq! (shift PHASE_RLP_TXN 6) TYPE_2_RLP_TXN_PHASE_NUMBER_6))
         (if-not-zero TYPE2 (eq! (shift PHASE_RLP_TXN 7) TYPE_2_RLP_TXN_PHASE_NUMBER_7))))

(defun (data_transfer)
  (begin (eq! (tx_type)                       (+ TYPE1 (* 2 TYPE2))) ;;(+ (* 0 TYPE0) (* 1 TYPE1) (* 2 TYPE2))
         (eq! (nonce)                         NONCE)
         (eq! (is_dep)                        IS_DEP)
         (eq! (value)                         VALUE)
         (eq! (gas_limit)                     GAS_LIMIT)
         (eq! (optional_to_addr_hi)           (* (- 1 IS_DEP) TO_HI))
         (eq! (optional_to_addr_lo)           (* (- 1 IS_DEP) TO_LO))
         (eq! (* (data_size) (- 1 IS_DEP))    CALL_DATA_SIZE)
         (eq! (* (data_size) IS_DEP)          INIT_CODE_SIZE)))

(defun (vanishing_data_cells)
  (begin (vanishes! (shift OUTGOING_HI 0))
         (vanishes! (shift OUTGOING_HI 2))
         (vanishes! (shift OUTGOING_HI 5))
         (if-zero TYPE2
                  (vanishes! (shift OUTGOING_HI 6)))))

;; is non-zero for the first row of each tx
(defun (first-row-of-new-transaction)
  (- ABS (prev ABS)))

(defconstraint   verticalization (:guard (first-row-of-new-transaction))
                 (begin (setting_phase_numbers)
                        (data_transfer)
                        (debug (vanishing_data_cells))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.9 EUC and WCP    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   euc-and-wcp-exclusivity ()
                 (or! (eq! EUC_FLAG 0) (eq! WCP_FLAG 0)))

(defun (small-call-to-LT row arg1 arg2)
  (begin (eq! (shift WCP_FLAG row) 1)
         (eq! (shift ARG_ONE_LO row) arg1)
         (eq! (shift ARG_TWO_LO row) arg2)
         (eq! (shift INST row) EVM_INST_LT)))

(defun (small-call-to-ISZERO row arg1)
  (begin (eq! (shift WCP_FLAG row) 1)
         (eq! (shift ARG_ONE_LO row) arg1)
         (eq! (shift INST row) EVM_INST_ISZERO)))

(defun (call-to-EUC row arg1 arg2)
  (begin (eq! (shift EUC_FLAG row) 1)
         (eq! (shift ARG_ONE_LO row) arg1)
         (eq! (shift ARG_TWO_LO row) arg2)))

(defun (small-call-to-LEQ    row-offset
                             arg1
                             arg2)
  (begin (eq!   (shift   WCP_FLAG     row-offset)   1)
         (eq!   (shift   ARG_ONE_LO   row-offset)   arg1)
         (eq!   (shift   ARG_TWO_LO   row-offset)   arg2)
         (eq!   (shift   INST         row-offset)   WCP_INST_LEQ)))

(defun (result-must-be-false row-offset) (vanishes! (shift RES row-offset)))
(defun (result-must-be-true  row-offset) (eq!       (shift RES row-offset) 1))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.9 Shared computations    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    comparison---nonce-must-not-exceed-EIP-2681-max-nonce    (:guard (first-row-of-new-transaction))
                  (begin
                    (small-call-to-LT    row-offset---nonce-comparison NONCE EIP2681_MAX_NONCE)
                    (result-must-be-true row-offset---nonce-comparison)))

(defconstraint    comparison---initial-balance-must-cover-value-plus-maximal-gas-cost    (:guard (first-row-of-new-transaction))
                  (begin
                    (small-call-to-LEQ    row-offset---initial-balance-comparison (+ (value) (* (max_fee) (gas_limit))) INITIAL_BALANCE)
                    (result-must-be-true  row-offset---initial-balance-comparison)))

(defun (upfront_gas_cost)
  (+   (*   TYPE0   (legacy_upfront_gas_cost))
       (*   TYPE1   (access_upfront_gas_cost))
       (*   TYPE2   (access_upfront_gas_cost))))
(defun (legacy_upfront_gas_cost)
  (+   (data_cost)
       GAS_CONST_G_TRANSACTION
       (* (is_dep) GAS_CONST_G_TX_CREATE)))
(defun (access_upfront_gas_cost)
  (+   (data_cost)
       GAS_CONST_G_TRANSACTION
       (* (is_dep)   GAS_CONST_G_TX_CREATE)
       (* (num_addr) GAS_CONST_G_ACCESS_LIST_ADRESS)
       (* (num_keys) GAS_CONST_G_ACCESS_LIST_STORAGE)))

(defconstraint    comparison---gas-limit-must-cover-upfront-gas-cost    (:guard (first-row-of-new-transaction))
                  (begin
                    (small-call-to-LEQ    row-offset---sufficient-gas-comparison (upfront_gas_cost) (gas_limit))
                    (result-must-be-true  row-offset---sufficient-gas-comparison)))

(defconstraint    integer-division---compute-upper-limit-for-refunds   (:guard (first-row-of-new-transaction))
                  (begin
                    (call-to-EUC    row-offset---upper-limit-refunds-comparison (execution_gas_cost) MAX_REFUND_QUOTIENT)))

(defun (execution_gas_cost) (- (gas_limit) GAS_LEFTOVER))
(defun (refund_limit)       (shift RES row-offset---upper-limit-refunds-comparison))

(defconstraint    comparison---final-refund-counter-vs-refund-limit    (:guard (first-row-of-new-transaction))
                  (small-call-to-LT    row-offset---effective-refund-comparison REF_CNT (refund_limit)))

(defun (get_full_refund) (force-bin (shift RES row-offset---effective-refund-comparison)))

(defconstraint    comparison---detect-empty-data-in-transaction    (:guard (first-row-of-new-transaction))
                  (small-call-to-ISZERO    row-offset---detecting-empty-call-data-comparison (data_size)))

(defun (nonzero-data-size) (force-bin (- 1 (shift RES row-offset---detecting-empty-call-data-comparison))))

(defconstraint    comparison---comparing-the-maximum-gas-price-against-the-basefee    (:guard (first-row-of-new-transaction))
                  (begin
                    (small-call-to-LEQ    row-offset---max-fee-and-basefee-comparison BASEFEE (maximal_gas_price))
                    (result-must-be-true  row-offset---max-fee-and-basefee-comparison)))

(defun (maximal_gas_price)   (+   (*   TYPE0   (gas_price))
                                  (*   TYPE1   (gas_price))
                                  (*   TYPE2   (max_fee))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; applicable only to type 2 transactions ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    comparison-for-type-2---comparing-max-fee-and-max-priority-fee    (:guard    (*   (first-row-of-new-transaction)   TYPE2))
                  (begin (small-call-to-LEQ    row-offset---max-fee-and-max-priority-fee-comparison (max_priority_fee) (max_fee))
                         (result-must-be-true  row-offset---max-fee-and-max-priority-fee-comparison)))

(defconstraint    comparison-for-type-2---computing-the-effective-gas-price         (:guard    (*   (first-row-of-new-transaction)   TYPE2))
                  (small-call-to-LEQ   row-offset---computing-effective-gas-price-comparison (+ (max_priority_fee) BASEFEE) (max_fee)))

(defun (get_full_tip) (force-bin (shift RES row-offset---computing-effective-gas-price-comparison)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    2.11 Setting certain variables  ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   setting-gas-initially-available (:guard (first-row-of-new-transaction))
                 (if-not-zero TYPE0
                              (eq! GAS_INITIALLY_AVAILABLE (- (gas_limit) (legacy_upfront_gas_cost)))
                              (eq! GAS_INITIALLY_AVAILABLE (- (gas_limit) (access_upfront_gas_cost)))))

(defconstraint   setting-gas-price (:guard (first-row-of-new-transaction))
                 (if-zero TYPE2
                          (eq! GAS_PRICE (gas_price))
                          (if-not-zero (get_full_tip)
                                       (eq! GAS_PRICE (+ BASEFEE (max_priority_fee)))
                                       (eq! GAS_PRICE (max_fee)))))

(defconstraint   setting-priority-fee-per-gas (:guard (first-row-of-new-transaction))
                 (eq! PRIORITY_FEE_PER_GAS (- GAS_PRICE BASEFEE)))

(defconstraint   setting-refund-effective (:guard (first-row-of-new-transaction))
                 (if-zero (get_full_refund)
                          (eq! REFUND_EFFECTIVE (+ GAS_LEFTOVER (refund_limit)))
                          (eq! REFUND_EFFECTIVE (+ GAS_LEFTOVER REFUND_COUNTER))))

(defconstraint   partially-setting-requires-evm-execution (:guard (first-row-of-new-transaction))
                 (if-not-zero IS_DEP
                              (eq! REQUIRES_EVM_EXECUTION (nonzero-data-size))))

(defconstraint   setting-copy-txcd (:guard (first-row-of-new-transaction))
                 (eq! COPY_TXCD
                      (* (- 1 IS_DEP) REQUIRES_EVM_EXECUTION (nonzero-data-size))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;    2.12 Verticalisation for RlpTxnRcpt    ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   verticalisation-rlp-txn-rcpt (:guard (first-row-of-new-transaction))
                 (begin
                   (eq! PHASE_RLP_TXNRCPT    RLP_RCPT_SUBPHASE_ID_TYPE)
                   (eq! OUTGOING_RLP_TXNRCPT (tx_type))
                   (eq! (next    PHASE_RLP_TXNRCPT)    RLP_RCPT_SUBPHASE_ID_STATUS_CODE)
                   (eq! (next    OUTGOING_RLP_TXNRCPT) STATUS_CODE)
                   (eq! (shift   PHASE_RLP_TXNRCPT     2) RLP_RCPT_SUBPHASE_ID_CUMUL_GAS)
                   (eq! (shift   OUTGOING_RLP_TXNRCPT  2) GAS_CUMULATIVE)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.13 Cumulative gas   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   cumulative-gas---vanishing-in-padding ()
                 (if-zero    ABS
                             (vanishes! GAS_CUMULATIVE)))

(defconstraint   cumulative-gas---initialization-at-block-start ()
                        (if-not (will-remain-constant! BLK)
                                ; BLK[i + 1] != BLK[i]
                                (eq! (next GAS_CUMULATIVE)
                                     (next (- GAS_LIMIT REFUND_EFFECTIVE)))))

(defconstraint   cumulative-gas---update-at-transaction-threshold ()
                 (if-not    (will-inc! BLK 1)
                            (if-not    (will-remain-constant! ABS)
                                                 ; BLK[i + 1] != 1 + BLK[i] && ABS[i+1] != ABS[i] i.e. BLK[i + 1] == BLK[i] && ABS[i+1] == ABS[i] +1
                                            (eq!    (next GAS_CUMULATIVE)
                                                    (+    GAS_CUMULATIVE (next (- GAS_LIMIT REFUND_EFFECTIVE)))))))

(defconstraint   cumulative-gas-comparison (:guard IS_LAST_TX_OF_BLOCK)
                 (if-not-zero (- ABS_TX_NUM (prev ABS_TX_NUM))
                              (if-zero TYPE0
                                       (begin (small-call-to-LEQ     NB_ROWS_TYPE_1
                                                                     GAS_CUMULATIVE
                                                                     BLOCK_GAS_LIMIT)
                                              (result-must-be-true   NB_ROWS_TYPE_1))
                                       (begin (small-call-to-LEQ     NB_ROWS_TYPE_0
                                                                     GAS_CUMULATIVE
                                                                     BLOCK_GAS_LIMIT)
                                              (result-must-be-true   NB_ROWS_TYPE_0)))))
