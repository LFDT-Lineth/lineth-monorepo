(module txnData)

(defconst 
  nROWS0                        6
  nROWS1                        7
  nROWS2                        7
  ;;
  G_transaction                 21000
  G_txcreate                    32000
  G_accesslistaddress           2400
  G_accessliststorage           1900
  ;;
  LT                            0x10
  ISZERO                        21
  ;; the following should be taken from rlpTxn constant
  common_rlp_txn_phase_number_0 1
  common_rlp_txn_phase_number_1 8
  common_rlp_txn_phase_number_2 3
  common_rlp_txn_phase_number_3 9
  common_rlp_txn_phase_number_4 10
  common_rlp_txn_phase_number_5 7
  type_0_rlp_txn_phase_number_6 4
  type_1_rlp_txn_phase_number_6 4
  type_1_rlp_txn_phase_number_7 11
  type_2_rlp_txn_phase_number_6 6
  type_2_rlp_txn_phase_number_7 11)

;; sum of transaction type flags
(defun (tx-type-sum)
  (+ TYPE0 TYPE1 TYPE2))

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
  (begin (= (tx-type-sum) 1)
         (if-not-zero TYPE0
                      (if-eq-else CT nROWS0 (will-inc! ABS 1) (will-inc! CT 1)))
         (if-not-zero TYPE1
                      (if-eq-else CT nROWS1 (will-inc! ABS 1) (will-inc! CT 1)))
         (if-not-zero TYPE2
                      (if-eq-else CT nROWS2 (will-inc! ABS 1) (will-inc! CT 1)))))

(defconstraint final-row (:domain {-1})
  (begin (= ABS ABS_MAX)
         (= BTC BTC_MAX)
         (= REL REL_MAX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.2 Constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (transaction-constant X)
  (if-not-zero CT
               (eq! X (prev X))))

(defconstraint constancies ()
  (begin (transaction-constant BTC)
         (transaction-constant REL)
         (transaction-constant FROM_HI)
         (transaction-constant FROM_LO)
         (transaction-constant NONCE)
         (transaction-constant IBAL)
         (transaction-constant VALUE)
         (transaction-constant TO_HI)
         (transaction-constant TO_LO)
         (transaction-constant IS_DEP)
         (transaction-constant GLIM)
         (transaction-constant IGAS)
         (transaction-constant GPRC)
         (transaction-constant BASEFEE)
         (transaction-constant CALL_DATA_SIZE)
         (transaction-constant INIT_CODE_SIZE)
         (transaction-constant TYPE0)
         (transaction-constant TYPE1)
         (transaction-constant TYPE2)
         (transaction-constant REQ_EVM)
         (transaction-constant COPY_TXCD_AT_INITIALISATION)
         (transaction-constant LEFTOVER_GAS)
         (transaction-constant REF_CNT)
         (transaction-constant REF_AMT)
         (transaction-constant CUM_GAS)
         (transaction-constant STATUS_CODE)
         (transaction-constant CFI)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    2.4 Batch numbers and transaction number    ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint total-number-constancies ()
  (begin (if-zero ABS
                  (begin (vanishes! ABS_MAX)
                         (vanishes! BTC_MAX)
                         (vanishes! REL_MAX))
                  (begin (will-remain-constant! ABS_MAX)
                         (will-remain-constant! BTC_MAX)))
         (if-not-zero (will-inc! BTC 1)
                      (will-remain-constant! REL_MAX))))

(defconstraint batch-num-increments ()
  (stamp-progression BTC))

(defconstraint batchNum-txNum-lexicographic ()
  (begin (if-zero ABS
                  (begin (vanishes! BTC)
                         (vanishes! REL)
                         (if-not-zero (will-remain-constant! ABS)
                                      (begin (eq! (next BTC) 1)
                                             (eq! (next REL) 1))))
                  (if-not-zero (will-remain-constant! ABS)
                               (if-not-zero (- REL_MAX REL)
                                            (begin (will-remain-constant! BTC)
                                                   (will-inc! REL 1))
                                            (begin (will-inc! BTC 1)
                                                   (= REL 1)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.5 Cumulative gas    ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;TODO reenable this constraint
;;(defconstraint cumulative-gas ()
;;  (begin (if-zero ABS
;;                  (vanishes! CUM_GAS))
;;         (if-not-zero (will-remain-constant! BTC)
;;                      ; BTC[i + 1] != BTC[i]
;;                      (eq! (next CUMULATIVE_CONSUMED_GAS)
;;                           (next (- GAS_LIMIT REFUND_AMOUNT))))
;;         (if-not-zero (and (will-inc! BTC 1) (will-remain-constant! ABS))
;;                      ; BTC[i + 1] != 1 + BTC[i] && ABS[i+1] != ABS[i] i.e. BTC[i + 1] == BTC[i] && ABS[i+1] == ABS[i] +1
;;                      (eq! (next CUM_GAS)
;;                           (+ CUM_GAS
;;                              (next (- GAS_LIMIT REFUND_AMOUNT)))))))
;;
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
  (begin (= (shift PHASE_RLP_TXN 0) common_rlp_txn_phase_number_0)
         (= (shift PHASE_RLP_TXN 1) common_rlp_txn_phase_number_1)
         (= (shift PHASE_RLP_TXN 2) common_rlp_txn_phase_number_2)
         (= (shift PHASE_RLP_TXN 3) common_rlp_txn_phase_number_3)
         (= (shift PHASE_RLP_TXN 4) common_rlp_txn_phase_number_4)
         (= (shift PHASE_RLP_TXN 5) common_rlp_txn_phase_number_5)
         ;;
         (if-not-zero TYPE0
                      (= (shift PHASE_RLP_TXN 6) type_0_rlp_txn_phase_number_6))
         ;;
         (if-not-zero TYPE1
                      (= (shift PHASE_RLP_TXN 6) type_1_rlp_txn_phase_number_6))
         (if-not-zero TYPE1
                      (= (shift PHASE_RLP_TXN 7) type_1_rlp_txn_phase_number_7))
         ;;
         (if-not-zero TYPE2
                      (= (shift PHASE_RLP_TXN 6) type_2_rlp_txn_phase_number_6))
         (if-not-zero TYPE2
                      (= (shift PHASE_RLP_TXN 7) type_2_rlp_txn_phase_number_7))))

(defun (data_transfer)
  (begin (= (tx_type) (+ TYPE1 TYPE2 TYPE2))
         (= (nonce) NONCE)
         (= (is_dep) IS_DEP)
         (= (value) VALUE)
         (= (optional_to_addr_hi)
            (* (- 1 IS_DEP) TO_HI))
         (= (optional_to_addr_lo)
            (* (- 1 IS_DEP) TO_LO))
         (if-zero IS_DEP
                  ;; IS_DEP == 0
                  (begin (= (data_size) CALL_DATA_SIZE)
                         (vanishes! INIT_CODE_SIZE))
                  ;; IS_DEP != 0
                  (begin (vanishes! CALL_DATA_SIZE)
                         (= (data_size) INIT_CODE_SIZE)
                         (= (gas_limit) GAS_LIMIT)))))

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
         (vanishing_data_cells)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.9 Comparisons    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; row i
(defun (sufficient_balance)
  (begin (= WCP_ARG_ONE_LO IBAL)
         (= WCP_ARG_TWO_LO
            (+ (value) (* (max_fee) (gas_limit))))
         (= WCP_INST LT)
         (vanishes! WCP_RES)))

(defun (upfront_gas_cost_of_transaction)
  (if-not-zero TYPE0
               ;; TYPE0 = 1
               (+ (data_cost) G_transaction (* (is_dep) G_txcreate))
               ;; TYPE0 = 0
               (+ (data_cost)
                  G_transaction
                  (* (is_dep) G_txcreate)
                  (* (num_addr) G_accesslistaddress)
                  (* (num_keys) G_accessliststorage))))

;; row i + 1
(defun (sufficient_gas_limit)
  (begin (= (shift WCP_ARG_ONE_LO 1) (gas_limit))
         (= (shift WCP_ARG_TWO_LO 1) (upfront_gas_cost_of_transaction))
         (= (shift WCP_INST 1) LT)
         (vanishes! (shift WCP_RES 1))))

;; epsilon is the remainder in the euclidean division of [T_g - g'] by 2
(defun (epsilon)
  (- (gas_limit)
     (+ LEFTOVER_GAS
        (* (shift WCP_ARG_TWO_LO 2) 2))))

;; row i + 2
(defun (upper_limit_for_refunds)
  (begin (= (shift WCP_ARG_ONE_LO 2) (- (gas_limit) LEFTOVER_GAS))
         ;;  (= (shift WCP_ARG_TWO_LO 2) ???) ;; unknown
         (= (shift WCP_INST 2) LT)
         (vanishes! (shift WCP_RES 2))
         (is-binary (epsilon))))

;; row i + 3
(defun (effective_refund)
  (begin (= (shift WCP_ARG_ONE_LO 3) REF_CNT)
         (= (shift WCP_ARG_TWO_LO 3) (shift WCP_ARG_TWO_LO 2))
         (= (shift WCP_INST 3) LT)
         ;;  (= (shift WCP_RES     3) ???) ;; unknown
         ))

;; row i + 4
(defun (is-zero-call-data)
  (begin (eq! (shift WCP_ARG_ONE_LO 4) CALL_DATA_SIZE)
         (eq! (shift WCP_INST 4) ISZERO)
         (eq! COPY_TXCD_AT_INITIALISATION
              (* REQUIRES_EVM_EXECUTION
                 (- 1 (shift WCP_RES 4))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; applicable only to type 2 transactions ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; row i + 5
(defun (type_2_comparing_max_fee_and_basefee)
  (begin (= (shift WCP_ARG_ONE_LO 5) (max_fee))
         (= (shift WCP_ARG_TWO_LO 5) BASEFEE)
         (= (shift WCP_INST 5) LT)
         (= (shift WCP_RES 5) 0)))

;; row i + 6
(defun (type_2_comparing_max_fee_and_max_priority_fee)
  (begin (= (shift WCP_ARG_ONE_LO 6) (max_fee))
         (= (shift WCP_ARG_TWO_LO 6) (max_priority_fee))
         (= (shift WCP_INST 6) LT)
         (= (shift WCP_RES 6) 0)))

;; row i + 7
(defun (type_2_computing_the_effective_gas_price)
  (begin (= (shift WCP_ARG_ONE_LO 7) (max_fee))
         (= (shift WCP_ARG_TWO_LO 7) (+ (max_priority_fee) BASEFEE))
         (= (shift WCP_INST 7) LT)
         ;;  (= (shift WCP_RES     6) ???) ;; unknown
         ))

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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    2.11 Gas and gas price constraints    ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint gas_and_gas_price (:guard (first-row-trigger))
  (begin  ;; constraining INIT_GAS
         (= IGAS
            (- (gas_limit) (shift WCP_ARG_TWO_LO 1)))
         ;; constraining REFUND_AMOUNT
         (if-zero (shift WCP_RES 3)
                  (= REF_AMT
                     (+ LEFTOVER_GAS (shift WCP_ARG_TWO_LO 2)))
                  (= REF_AMT (+ LEFTOVER_GAS REF_CNT)))
         ;; constraining GAS_PRICE
         (if-zero TYPE2
                  (= GAS_PRICE (gas_price))
                  (if-zero (shift WCP_RES 7)
                           (= GAS_PRICE (+ (max_priority_fee) BASEFEE))
                           (= GAS_PRICE (max_fee))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;    2.11 Verticalisation for RlpTxnRcpt    ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint verticalisation-rlp-txn-rcpt (:guard (first-row-trigger))
  (begin (eq! PHASE_RLP_TXNRCPT RLPRECEIPT_SUBPHASE_ID_TYPE)
         (eq! OUTGOING_RLP_TXNRCPT (tx_type))
         (eq! (next PHASE_RLP_TXNRCPT) RLPRECEIPT_SUBPHASE_ID_STATUS_CODE)
         (eq! (next OUTGOING_RLP_TXNRCPT) STATUS_CODE)
         (eq! (shift PHASE_RLP_TXNRCPT 2) RLPRECEIPT_SUBPHASE_ID_CUMUL_GAS)
         (eq! (shift OUTGOING_RLP_TXNRCPT 2) CUMULATIVE_CONSUMED_GAS)))


