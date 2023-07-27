(module txn_data)

(defconst
	nROWS0 6
	nROWS1 7
	nROWS2 7)

;; sum of transaction type flags
(defun (tx_type_sum) (+ TYPE0 TYPE1 TYPE2))

;; constraint imposing that STAMP[i + 1] ∈ { STAMP[i], 1 + STAMP[i] }
(depurefun (stamp_progression STAMP)
	   (vanishes! (*
			(will-remain-constant STAMP)
			(will-inc STAMP 1)))

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
			    (if-not-zero (will-remain-constant ABS)
					 (begin
					   (= BTC 1)
					   (= REL 1))))
			  (if-not-zero (will-remain-constant ABS)
				       (if-not-zero (- REL_MAX REL)
						    (begin
						      (will-remain-constant! BTC)
						      (will-inc REL 1))
						    (begin
						      (will-inc BTC 1)
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
					   (will-eq 
					     CUM_GAS
					     (- GLIM REF_AMT))))))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;    2.6 Aliases    ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun (tx_type)              (DATA_LO))
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
