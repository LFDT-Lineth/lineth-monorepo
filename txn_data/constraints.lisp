(module txn_data)

(defconst
	nROWS0                    6
	nROWS1                    7
	nROWS2                    7
	;;
	G_transaction         21000
	G_txcreate            32000
	G_accesslistaddress    2400
	G_accessliststorage    1900
	;;
	LT                     0x10
	)

;; sum of transaction type flags
(defun (tx_type_sum) (+ TYPE0 TYPE1 TYPE2))

;; constraint imposing that STAMP[i + 1] ∈ { STAMP[i], 1 + STAMP[i] }
(defpurefun (stamp_progression STAMP)
	   (vanishes! (*
			(will-remain-constant! STAMP)
			(will-inc! STAMP 1))))

;; TODO: better solution for the zero column ?
(defconstraint zerocol-must-be-the-zero-column ()
	       (vanishes! ZEROCOL)) 

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 Heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint first_row (:domain {0})
	       (vanishes! ABS))

(defconstraint padding () 
	       (if-zero ABS
			(begin
			  (vanishes! CT)
			  (vanishes! (tx_type_sum)))))

(defconstraint abs_tx_num_increments () (stamp_progression ABS))

(defconstraint heartbeat (:guard ABS)
	       (begin
		 (= (tx_type_sum) 1)
		 (if-not-zero TYPE0
			      (if-eq-else CT nROWS0
					  (will-inc! ABS 1)
					  (will-inc! CT  1)))
		 (if-not-zero TYPE1
			      (if-eq-else CT nROWS1
					  (will-inc! ABS 1)
					  (will-inc! CT  1)))
		 (if-not-zero TYPE2
			      (if-eq-else CT nROWS2
					  (will-inc! ABS 1)
					  (will-inc! CT  1)))
		 ))


(defconstraint final_row (:domain {-1})
	       (begin
		 (= ABS ABS_MAX)
		 (= BTC BTC_MAX)
		 (= REL REL_MAX)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.2 Constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (transaction-constant X) (if-not-zero (- ABS (+ 1 (prev ABS))) (= X (prev X))))

(defconstraint constancies ()
	       (begin
		 (transaction-constant BTC)
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
		 (transaction-constant DATA_SIZE)
		 (transaction-constant TYPE0)
		 (transaction-constant TYPE1)
		 (transaction-constant TYPE2)
		 (transaction-constant REQ_EVM)
		 (transaction-constant LEFTOVER_GAS)
		 (transaction-constant REF_CNT)
		 (transaction-constant REF_AMT)
		 (transaction-constant CUM_GAS)
		 (transaction-constant STATUS_CODE)
		 (transaction-constant CFI)))   

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.3 Binary constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint binary_constraints ()
	       (begin
		 (is-binary TYPE0)
		 (is-binary TYPE1)
		 (is-binary TYPE2)
		 (is-binary IS_DEP)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    2.4 Batch numbers and transaction number    ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint total_number_constancies ()
	       (begin
		 (if-zero ABS
			(begin
			  (vanishes! ABS_MAX)
			  (vanishes! BTC_MAX)
			  (vanishes! REL_MAX))
			(begin
			  (will-remain-constant! ABS_MAX)
			  (will-remain-constant! BTC_MAX)))
		 (if-not-zero (will-inc! BTC 1)
			      (will-remain-constant! REL_MAX))
		 ))

(defconstraint batch_num_increments () (stamp_progression BTC))

(defconstraint batchNum_txNum_lexicographic ()
	       (begin
		 (if-zero ABS
			  (begin
			    (vanishes! BTC)
			    (vanishes! REL)
			    (if-not-zero (will-remain-constant! ABS)
					 (begin
					   (= BTC 1)
					   (= REL 1))))
			  (if-not-zero (will-remain-constant! ABS)
				       (if-not-zero (- REL_MAX REL)
						    (begin
						      (will-remain-constant! BTC)
						      (will-inc! REL 1))
						    (begin
						      (will-inc! BTC 1)
						      (= REL 1))
						    )))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.5 Cumulative gas    ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

			      
(defconstraint cumulative_gas ()
	       (begin
		 (if-zero ABS (vanishes! CUM_GAS))
		 (if-not-zero (will-remain-constant! BTC)
			      ; BTC[i + 1] != BTC[i]
			      (will-remain-constant! CUM_GAS))
		 (if-not-zero (will-inc! BTC 1)
			      ; BTC[i + 1] != 1 + BTC[i] i.e. BTC[i + 1] == BTC[i]
			      (if-not-zero (will-remain-constant! ABS)
					   (will-eq!
					     CUM_GAS
					     (- GLIM REF_AMT))))))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;    2.6 Aliases    ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx_type)              DATA_LO)
(defun (to_addr_hi)           (shift DATA_HI 1))
(defun (to_addr_lo)           (shift DATA_LO 1))
(defun (nonce)                (shift DATA_LO 2))
(defun (is_dep)               (shift DATA_HI 3))
(defun (value)                (shift DATA_LO 3))
(defun (data_cost)            (shift DATA_HI 4))
(defun (data_size)            (shift DATA_LO 4))
(defun (gas_limit)            (shift DATA_LO 5))
(defun (gas_price)            (shift DATA_HI 6))
(defun (max_priority_fee)     (shift DATA_LO 6))
(defun (max_fee)              (shift DATA_LO 6))
(defun (num_keys)             (shift DATA_HI 7))
(defun (num_addr)             (shift DATA_LO 7))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    2.8 Verticalization    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (setting_phase_numbers)
  (begin
    (vanishes! PHASE)
;;  (= (shift PHASE 0) 0)
    (= (shift PHASE 1) 7)
    (= (shift PHASE 2) 2)
    (= (shift PHASE 3) 8)
    (= (shift PHASE 4) 9)
    (= (shift PHASE 5) 6)
    ;;
    (if-not-zero TYPE0 (= (shift PHASE 6)  3))
    ;;
    (if-not-zero TYPE1 (= (shift PHASE 6)  3))
    (if-not-zero TYPE1 (= (shift PHASE 7) 10))
    ;;
    (if-not-zero TYPE2 (= (shift PHASE 6)  5))
    (if-not-zero TYPE2 (= (shift PHASE 7) 10))
    ))

(defun (data_transfer)
  (begin
    (= (tx_type)         (+ TYPE1 TYPE2 TYPE2))
    (= (nonce)           NONCE)
    (= (is_dep)          IS_DEP)
    (= (value)           VALUE)
    (= (data_size)       DATA_SIZE)
    (= (gas_limit)       GAS_LIMIT)
    (if-zero IS_DEP
	     (begin
	       (= (to_addr_hi) TO_HI)
	       (= (to_addr_lo) TO_LO)))
    ))

(defun (vanishing_data_cells)
  (begin
    (vanishes! DATA_HI)
;;  (vanishes! (shift DATA_HI 0))
    (vanishes! (shift DATA_HI 2))
    (vanishes! (shift DATA_HI 5))
    (if-zero TYPE2
	         (vanishes! (shift DATA_HI 6)))
    ))

(defconstraint verticalization (:guard (remained-constant! ABS))
				       (begin
					 (setting_phase_numbers)
					 (data_transfer)
					 (vanishing_data_cells)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.9 Comparisons    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; row i
(defun (sufficient_balance)
  (begin
    (= WCP_ARG_ONE_LO      IBAL)
    (= WCP_ARG_TWO_LO      (+ (value) (* (max_fee) (gas_limit))))
    (= WCP_INST            LT)
    (vanishes! WCP_RES_LO)
    ))

(defun (upfront_gas_cost_of_transaction)
  (if-not-zero TYPE0
	       ;; TYPE0 = 1
	       (+   (data_cost)
		    G_transaction
		    (* (is_dep) G_txcreate))
	       ;; TYPE0 = 0
	       (+   (data_cost)
		    G_transaction
		    (* (is_dep) G_txcreate)
		    (* (num_addr) G_accesslistaddress)
		    (* (num_keys) G_accessliststorage))))


;; row i + 1
(defun (sufficient_gas_limit)
  (begin
    (= (shift WCP_ARG_ONE_LO 1) (gas_limit))
    (= (shift WCP_ARG_TWO_LO 1) (upfront_gas_cost_of_transaction))
    (= (shift WCP_INST 1) LT)
    (vanishes! (shift WCP_RES_LO 1))
    ))

;; epsilon is the remainder in the euclidean division of [T_g - g'] by 2
(defun (epsilon) (- (gas_limit)
		    LEFTOVER_GAS
		    (shift WCP_ARG_TWO_LO 2)
		    (shift WCP_ARG_TWO_LO 2)))

;; row i + 2
(defun (upper_limit_for_refunds)
  (begin
    (= (shift WCP_ARG_ONE_LO 2) (- (gas_limit) LEFTOVER_GAS))
;;  (= (shift WCP_ARG_TWO_LO 2) ???) ;; unknown
    (= (shift WCP_INST       2) LT)
    (vanishes! (shift WCP_RES_LO 2))
    (is-binary (epsilon))
    ))

;; row i + 3
(defun (effective_refund)
  (begin
    (= (shift WCP_ARG_ONE_LO 3) REF_CNT)
    (= (shift WCP_ARG_TWO_LO 3) (shift WCP_ARG_TWO_LO 2))
    (= (shift WCP_INST       3) LT)
;;  (= (shift WCP_RES_LO     3) ???) ;; unknown
    ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; applicable only to type 2 transactions ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; row i + 4
(defun (type_2_comparing_max_fee_and_basefee)
  (begin
    (= (shift WCP_ARG_ONE_LO 4) (max_fee))
    (= (shift WCP_ARG_TWO_LO 4) BASEFEE)
    (= (shift WCP_INST       4) LT)
    (= (shift WCP_RES_LO     4) 0)
    ))

;; row i + 5
(defun (type_2_comparing_max_fee_and_max_priority_fee)
  (begin
    (= (shift WCP_ARG_ONE_LO 5) (max_fee))
    (= (shift WCP_ARG_TWO_LO 5) (max_priority_fee))
    (= (shift WCP_INST       5) LT)
    (= (shift WCP_RES_LO     5) 0)
    ))

;; row i + 6
(defun (type_2_computing_the_effective_gas_price)
  (begin
    (= (shift WCP_ARG_ONE_LO 6) (max_fee))
    (= (shift WCP_ARG_TWO_LO 6) (+ (max_priority_fee)
				   BASEFEE))
    (= (shift WCP_INST       6) LT)
;;  (= (shift WCP_RES_LO     6) ???) ;; unknown
    ))

(defconstraint comparisons (:guard (remained-constant! ABS))
				   (begin
				     (sufficient_balance)
				     (sufficient_gas_limit)
				     (upper_limit_for_refunds)
				     (effective_refund)
				     (if-not-zero TYPE2
						  (begin
						  (type_2_comparing_max_fee_and_basefee)
						  (type_2_comparing_max_fee_and_max_priority_fee)
						  (type_2_computing_the_effective_gas_price)
						  ))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    2.11 Gas and gas price constraints    ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint gas_and_gas_price (:guard (remained-constant! ABS))
	       (begin
		 ;; constraining INIT_GAS
		 (= IGAS (- (gas_limit) (shift WCP_ARG_TWO_LO 1)))
		 ;; constraining REFUND_AMOUNT
		 (if-zero (shift WCP_RES_LO 3)
			  (= REF_AMT (+ LEFTOVER_GAS (shift WCP_ARG_TWO_LO 2)))
			  (= REF_AMT (+ LEFTOVER_GAS REF_CNT)))
		 ;; constraining GAS_PRICE
		 (if-zero TYPE2
			  (= GAS_PRICE (gas_price))
			  (if-zero (shift WCP_RES_LO 6)
				   (= GAS_PRICE (+ (max_priority_fee) BASEFEE))
				   (= GAS_PRICE (max_fee))))
		 ))

