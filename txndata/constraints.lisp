(module txndata)

(defpurefun (if-not-eq A B then else)
  (if-not-zero (- A B)
               then
               else))

;; sum of transaction type flags
(defun (tx-type-sum)
  (force-bool (+ TYPE0 TYPE1 TYPE2)))

;; constraint imposing that STAMP[i + 1] ∈ { STAMP[i], 1 + STAMP[i] }
(defpurefun (stamp-progression STAMP)
  (vanishes! (any! (will-remain-constant! STAMP) (will-inc! STAMP 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 Heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! ABS))

(defconstraint padding-is-padding ()
  (if-zero ABS
           (begin (debug (vanishes! CT))
                  (vanishes! (weight_sum))))) ;;TODO: useless, but in the spec

(defconstraint padding ()
  (if-zero ABS
           (begin (vanishes! CT)
                  (vanishes! (tx-type-sum)))))

(defconstraint abs-tx-num-increments ()
  (stamp-progression ABS))

(defconstraint new-stamp-reboot-ct ()
  (if-not-zero (will-remain-constant! ABS)
               (vanishes! (next CT))))

(defconstraint heartbeat (:guard ABS)
  (begin (eq! (tx-type-sum) 1)
         (if-zero TYPE0
                  (if-eq-else CT (- (+ NB_ROWS_TYPE_1 IS_LAST_TX_OF_BLOCK) 1)
                              (will-inc! ABS 1)
                              (will-inc! CT 1))
                  (if-eq-else CT (- (+ NB_ROWS_TYPE_0 IS_LAST_TX_OF_BLOCK) 1)
                              (will-inc! ABS 1)
                              (will-inc! CT 1)))))

(defconstraint final-row (:domain {-1})
  (begin (eq! ABS ABS_MAX)
         (eq! REL REL_MAX)
         (if-not-zero TYPE0
                      (eq! CT NB_ROWS_TYPE_0))
         (if-not-zero TYPE1
                      (eq! CT NB_ROWS_TYPE_1))
         (if-not-zero TYPE2
                      (eq! CT NB_ROWS_TYPE_2))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.2 Constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (transaction-constant X)
  (if-not-zero CT
               (eq! X (prev X))))

(defun (weight_sum)
  (+ (* (^ 2 0) IS_DEP)
     (* (^ 2 1) TYPE0)
     (* (^ 2 2) TYPE1)
     (* (^ 2 3) TYPE2)
     (* (^ 2 4) REQUIRES_EVM_EXECUTION)
     (* (^ 2 5) COPY_TXCD)
     (* (^ 2 6) STATUS_CODE)))

(defconstraint constancies ()
  (begin (transaction-constant FROM_HI)
         (transaction-constant FROM_LO)
         (transaction-constant NONCE)
         (transaction-constant VALUE)
         (transaction-constant GLIM)
         (transaction-constant TO_HI)
         (transaction-constant TO_LO)
         (transaction-constant CALL_DATA_SIZE)
         (transaction-constant INIT_CODE_SIZE)
         (debug (transaction-constant IGAS))
         (debug (transaction-constant PRIORITY_FEE_PER_GAS))
         (debug (transaction-constant GAS_PRICE))
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
         (transaction-constant (weight_sum))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    2.4 Batch numbers and transaction number    ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint total-number-constancies ()
  (begin (if-not-zero ABS
                      (will-remain-constant! ABS_MAX)
                      ;; (begin (vanishes! ABS_MAX)
                      ;;        (vanishes! BLK_MAX)
                      ;;        (vanishes! REL_MAX)
                      ;;        (vanishes! BLK)
                      ;;        (vanishes! REL))
                      )
         (if-not-zero (will-inc! BLK 1)
                      (will-remain-constant! REL_MAX))))

(defconstraint batch-num-increments ()
  (stamp-progression BLK))

(defconstraint batchNum-txNum-lexicographic ()
  (begin (if-zero ABS
                  (begin (vanishes! BLK)
                         (vanishes! REL)
                         (if-not-zero (will-remain-constant! ABS)
                                      (begin (eq! (next BLK) 1)
                                             (eq! (next REL) 1))))
                  (if-not-zero (will-remain-constant! ABS)
                               (if-not-eq REL_MAX
                                          REL
                                          (begin (will-remain-constant! BLK)
                                                 (will-inc! REL 1))
                                          (begin (will-inc! BLK 1)
                                                 (will-eq! REL 1)))))))

(defconstraint set-last-tx-of-block-flag (:guard ABS_TX_NUM)
  (if-eq-else REL_TX_NUM REL_TX_NUM_MAX
              (eq! IS_LAST_TX_OF_BLOCK 1)
              (vanishes! IS_LAST_TX_OF_BLOCK)))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;    2.6 Aliases    ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;
(defun (tx_type)
  (shift OUTGOING_LO 0))

(defun (optional_to_addr_hi)
  (shift OUTGOING_HI 1))

(defun (optional_to_addr_lo)
  (shift OUTGOING_LO 1))

(defun (nonce)
  (shift OUTGOING_LO 2))

(defun (is_dep)
  (shift OUTGOING_HI 3))

(defun (value)
  (shift OUTGOING_LO 3))

(defun (data_cost)
  (shift OUTGOING_HI 4))

(defun (data_size)
  (shift OUTGOING_LO 4))

(defun (gas_limit)
  (shift OUTGOING_LO 5))

(defun (gas_price)
  (shift OUTGOING_LO 6))

(defun (max_priority_fee)
  (shift OUTGOING_HI 6))

(defun (max_fee)
  (shift OUTGOING_LO 6))

(defun (num_keys)
  (shift OUTGOING_HI 7))

(defun (num_addr)
  (shift OUTGOING_LO 7))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    2.8 Verticalization    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (setting_phase_numbers)
  (begin (= (shift PHASE_RLP_TXN 0) COMMON_RLP_TXN_PHASE_NUMBER_0)
         (= (shift PHASE_RLP_TXN 1) COMMON_RLP_TXN_PHASE_NUMBER_1)
         (= (shift PHASE_RLP_TXN 2) COMMON_RLP_TXN_PHASE_NUMBER_2)
         (= (shift PHASE_RLP_TXN 3) COMMON_RLP_TXN_PHASE_NUMBER_3)
         (= (shift PHASE_RLP_TXN 4) COMMON_RLP_TXN_PHASE_NUMBER_4)
         (= (shift PHASE_RLP_TXN 5) COMMON_RLP_TXN_PHASE_NUMBER_5)
         ;;
         (if-not-zero TYPE0
                      (= (shift PHASE_RLP_TXN 6) TYPE_0_RLP_TXN_PHASE_NUMBER_6))
         ;;
         (if-not-zero TYPE1
                      (= (shift PHASE_RLP_TXN 6) TYPE_1_RLP_TXN_PHASE_NUMBER_6))
         (if-not-zero TYPE1
                      (= (shift PHASE_RLP_TXN 7) TYPE_1_RLP_TXN_PHASE_NUMBER_7))
         ;;
         (if-not-zero TYPE2
                      (= (shift PHASE_RLP_TXN 6) TYPE_2_RLP_TXN_PHASE_NUMBER_6))
         (if-not-zero TYPE2
                      (= (shift PHASE_RLP_TXN 7) TYPE_2_RLP_TXN_PHASE_NUMBER_7))))

(defun (data_transfer)
  (begin (eq! (tx_type)
              (+ TYPE1 (* 2 TYPE2))) ;;(+ (* 0 TYPE0) (* 1 TYPE1) (* 2 TYPE2))
         (eq! (nonce) NONCE)
         (eq! (is_dep) IS_DEP)
         (eq! (value) VALUE)
         (eq! (optional_to_addr_hi)
              (* (- 1 IS_DEP) TO_HI))
         (eq! (optional_to_addr_lo)
              (* (- 1 IS_DEP) TO_LO))
         (eq! (* (data_size) (- 1 IS_DEP))
              CALL_DATA_SIZE)
         (eq! (* (data_size) IS_DEP) INIT_CODE_SIZE)))

(defun (vanishing_data_cells)
  (begin (vanishes! (shift OUTGOING_HI 0))
         (vanishes! (shift OUTGOING_HI 2))
         (vanishes! (shift OUTGOING_HI 5))
         (if-zero TYPE2
                  (vanishes! (shift OUTGOING_HI 6)))))

;; is non-zero for the first row of each tx
(defun (first-row-trigger)
  (- ABS (prev ABS)))

(defconstraint verticalization (:guard (first-row-trigger))
  (begin (setting_phase_numbers)
         (data_transfer)
         (debug (vanishing_data_cells))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.9 EUC and WCP    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint euc-and-wcp-exclusivity ()
  (vanishes! (* EUC_FLAG WCP_FLAG)))

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

(defun (small-call-to-LEQ row arg1 arg2)
  (begin (eq! (shift WCP_FLAG row) 1)
         (eq! (shift ARG_ONE_LO row) arg1)
         (eq! (shift ARG_TWO_LO row) arg2)
         (eq! (shift INST row) WCP_INST_LEQ)))

(defun (result-must-be-false row)
  (vanishes! (shift RES row)))

(defun (result-must-be-true row)
  (eq! (shift RES row) 1))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.9 Comparisons    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; row i
(defun (sufficient_balance)
  (begin (small-call-to-LT 0
                           INITIAL_BALANCE
                           (+ (value) (* (max_fee) (gas_limit))))
         (result-must-be-false 0)))

;; row i + 1
(defun (upfront_gas_cost_of_transaction)
  (if-not-zero TYPE0
               ;; TYPE0 = 1
               (legacy_upfront_gas_cost)
               ;; TYPE0 = 0
               (access_upfront_gas_cost)))

(defun (legacy_upfront_gas_cost)
  (+ (data_cost) GAS_CONST_G_TRANSACTION (* (is_dep) GAS_CONST_G_TX_CREATE)))

(defun (access_upfront_gas_cost)
  (+ (data_cost)
     GAS_CONST_G_TRANSACTION
     (* (is_dep) GAS_CONST_G_TX_CREATE)
     (* (num_addr) GAS_CONST_G_ACCESS_LIST_ADRESS)
     (* (num_keys) GAS_CONST_G_ACCESS_LIST_STORAGE)))

(defun (sufficient_gas_limit)
  (begin (small-call-to-LT 1 (gas_limit) (upfront_gas_cost_of_transaction))
         (result-must-be-false 1)))

;; row i + 2
;; epsilon is the remainder in the euclidean division of [T_g - g'] by 2
(defun (execution-gas-cost)
  (- (gas_limit) GAS_LEFTOVER))

(defun (upper_limit_for_refunds)
  (begin (call-to-EUC 2 (execution-gas-cost) 2)))

(defun (refund_limit)
  (shift RES 2))

;; row i + 3
(defun (effective_refund)
  (small-call-to-LT 3 REF_CNT (refund_limit)))

(defun (get_full_refund)
  (force-bool (shift RES 3)))

;; row i + 4
(defun (is-zero-call-data)
  (small-call-to-ISZERO 4 (data_size)))

(defun (nonzero_data_size)
  (force-bool (- 1 (shift RES 4))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; applicable only to type 2 transactions ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; row i + 5
(defun (type_2_comparing_max_fee_and_basefee)
  (begin (small-call-to-LT 5 (max_fee) BASEFEE)
         (result-must-be-false 5)))

;; row i + 6
(defun (type_2_comparing_max_fee_and_max_priority_fee)
  (begin (small-call-to-LT 6 (max_fee) (max_priority_fee))
         (result-must-be-false 6)))

;; row i + 7
(defun (type_2_computing_the_effective_gas_price)
  (small-call-to-LT 7 (max_fee) (+ (max_priority_fee) BASEFEE)))

(defun (get_full_tip)
  (force-bool (- 1 (shift RES 7))))

(defconstraint comparisons (:guard (first-row-trigger))
  (begin (sufficient_balance)
         (sufficient_gas_limit)
         (upper_limit_for_refunds)
         (effective_refund)
         (is-zero-call-data)
         (if-not-zero TYPE2
                      (begin (type_2_comparing_max_fee_and_basefee)
                             (type_2_comparing_max_fee_and_max_priority_fee)
                             (type_2_computing_the_effective_gas_price)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    2.11 Setting certain variables  ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint setting-gas-init (:guard (first-row-trigger))
  (if-not-zero TYPE0
               (eq! GAS_INITIALLY_AVAILABLE (- (gas_limit) (legacy_upfront_gas_cost)))
               (eq! GAS_INITIALLY_AVAILABLE (- (gas_limit) (access_upfront_gas_cost)))))

(defconstraint setting-gas-price (:guard (first-row-trigger))
  (if-zero TYPE2
           (eq! GAS_PRICE (gas_price))
           (if-not-zero (get_full_tip)
                        (eq! GAS_PRICE (+ BASEFEE (max_priority_fee)))
                        (eq! GAS_PRICE (max_fee)))))

(defconstraint setting-priority-fee-per-gas (:guard (first-row-trigger))
  (if-zero TYPE2
           (eq! PRIORITY_FEE_PER_GAS (gas_price))
           (eq! PRIORITY_FEE_PER_GAS (- (gas_price) BASEFEE))))

(defconstraint setting-refund-effective (:guard (first-row-trigger))
  (if-zero (get_full_refund)
           (eq! REFUND_EFFECTIVE (+ GAS_LEFTOVER (refund_limit)))
           (eq! REFUND_EFFECTIVE (+ GAS_LEFTOVER REFUND_COUNTER))))

(defconstraint partially-setting-requires-evm (:guard (first-row-trigger))
  (if-not-zero IS_DEP
               (eq! REQUIRES_EVM_EXECUTION (nonzero_data_size))))

(defconstraint setting-copy-txcd (:guard (first-row-trigger))
  (eq! COPY_TXCD
       (* (- 1 IS_DEP) REQUIRES_EVM_EXECUTION (nonzero_data_size))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;    2.12 Verticalisation for RlpTxnRcpt    ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint verticalisation-rlp-txn-rcpt (:guard (first-row-trigger))
  (begin (eq! PHASE_RLP_TXNRCPT RLP_RCPT_SUBPHASE_ID_TYPE)
         (eq! OUTGOING_RLP_TXNRCPT (tx_type))
         (eq! (next PHASE_RLP_TXNRCPT) RLP_RCPT_SUBPHASE_ID_STATUS_CODE)
         (eq! (next OUTGOING_RLP_TXNRCPT) STATUS_CODE)
         (eq! (shift PHASE_RLP_TXNRCPT 2) RLP_RCPT_SUBPHASE_ID_CUMUL_GAS)
         (eq! (shift OUTGOING_RLP_TXNRCPT 2) GAS_CUMULATIVE)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.13 Cumulative gas   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint cumulative-gas ()
  (begin (if-zero ABS
                  (vanishes! GAS_CUMULATIVE))
         (if-not-zero (will-remain-constant! BLK)
                      ; BLK[i + 1] != BLK[i]
                      (eq! (next GAS_CUMULATIVE)
                           (next (- GAS_LIMIT REFUND_EFFECTIVE))))
         (if-not-zero (and (will-inc! BLK 1) (will-remain-constant! ABS))
                      ; BLK[i + 1] != 1 + BLK[i] && ABS[i+1] != ABS[i] i.e. BLK[i + 1] == BLK[i] && ABS[i+1] == ABS[i] +1
                      (eq! (next GAS_CUMULATIVE)
                           (+ GAS_CUMULATIVE
                              (next (- GAS_LIMIT REFUND_EFFECTIVE)))))))

(defconstraint cumulative-gas-comparaison (:guard IS_LAST_TX_OF_BLOCK)
  (if-not-zero (- ABS_TX_NUM (prev ABS_TX_NUM))
               (if-zero TYPE0
                        (begin (small-call-to-LEQ NB_ROWS_TYPE_1 GAS_CUMULATIVE BLOCK_GAS_LIMIT)
                               (result-must-be-true NB_ROWS_TYPE_1))
                        (begin (small-call-to-LEQ NB_ROWS_TYPE_0 GAS_CUMULATIVE BLOCK_GAS_LIMIT)
                               (result-must-be-true NB_ROWS_TYPE_0)))))


