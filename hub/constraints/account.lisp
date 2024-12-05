(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;  6.1 Specialized-rows ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Nonce constraintes

(defun (same_nonce_h)
 (eq! NONCE NONCE_NEW))

(defun (increment_nonce_h)
 (eq! NONCE_NEW (+ NONCE 1)))

(defun (decrement_nonce_h)
 (eq! NONCE_NEW (- NONCE 1)))

(defun (undo_account_nonce_update_v)
 (begin
  (eq! NONCE (- (prev NONCE_NEW) 1))
  (eq! NONCE_NEW (prev NONCE))))

(defun (undo_previous_account_nonce_update_v)
 (begin
  (eq! NONCE (- (shift NONCE_NEW -2) 1))
  (eq! NONCE_NEW (shift NONCE -2))))

;; Balance constraints
(defun (same_balance_h)
 (eq! BALANCE_NEW BALANCE))

(defun (undo_account_balance_update_v)
 (begin
  (eq! BALANCE (prev BALANCE_NEW))
  (eq! BALANCE_NEW (prev BALANCE))))

(defun (undo_previous_account_balance_update_v)
 (begin
  (eq! BALANCE (shift BALANCE_NEW -2))
  (eq! BALANCE_NEW (shift BALANCE -2))))

;; Warmth constraints
(defun (same_account_warmth_h)
 (eq! WARMTH_NEW WARMTH))

(defun (turn_on_account_warmth_h)
 (eq! WARMTH_NEW 1))

(defun (undo_account_warmth_update_v)
 (begin
  (eq! WARMTH (prev WARMTH_NEW))
  (eq! WARMTH_NEW (prev WARMTH))))

;; Code constraints
(defun (same_code_size_h)
 (eq! CODE_SIZE_NEW CODE_SIZE))

(defun (same_code_hash_h)
 (begin
  (eq! CODE_HASH_HI_NEW CODE_HASH_HI)
  (eq! CODE_HASH_LO_NEW CODE_HASH_LO)))

(defun (same_code_h)
 (begin
  (same_code_size_h)
  (same_code_hash_h)))

(defun (undo_code_size_update_v)
 (begin
  (eq! CODE_SIZE (prev CODE_SIZE_NEW))
  (eq! CODE_SIZE_NEW (prev CODE_SIZE))))

(defun (undo_code_hash_update_v)
 (begin
  (eq! CODE_HASH_HI (prev CODE_HASH_HI_NEW))
  (eq! CODE_HASH_HI_NEW (prev CODE_HASH_HI))
  (eq! CODE_HASH_LO (prev CODE_HASH_LO_NEW))
  (eq! CODE_HASH_LO_NEW (prev CODE_HASH_LO))))

;; Deployment status constraints
(defun (same_dep_number_h)
 (eq! DEPLOYMENT_NUMBER_NEW DEPLOYMENT_NUMBER))

(defun (same_dep_status_h)
 (eq! DEPLOYMENT_STATUS_NEW DEPLOYMENT_STATUS))

(defun (same_dep_num_and_status_h)
 (begin
  (same_dep_number_h)
  (same_dep_status_h)))

(defun (undo_dep_number_update_v)
 (begin
  (eq! DEPLOYMENT_NUMBER (prev DEPLOYMENT_NUMBER_NEW))
  (eq! DEPLOYMENT_NUMBER_NEW (prev DEPLOYMENT_NUMBER))))

(defun (undo_dep_status_update_v)
 (begin
  (eq! DEPLOYMENT_STATUS (prev DEPLOYMENT_STATUS_NEW))
  (eq! DEPLOYMENT_STATUS_NEW (prev DEPLOYMENT_STATUS))))

(defun (undo_dep_status_and_number_update_v)
 (begin
  (undo_dep_number_update_v)
  (undo_dep_status_update_v)))

(defun (increment_dep_number_h)
 (eq! DEPLOYMENT_NUMBER_NEW (+ DEPLOYMENT_NUMBER 1)))

(defun (fresh_new_dep_num_and_status_h)
 (begin
  (vanishes! NONCE_NEW)
  (vanishes CODE_SIZE_NEW)
  (vanishes! HAS_CODE_NEW)
  (debug (eq! CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))
  (increment_dep_number_h)
  (vanishes! DEPLOYMENT_STATUS_NEW)))

(defun (dep_num_and_status_update_for_deployment_with_code_h)
 (begin
  (increment_dep_number_h)
  (eq! DEPLOYMENT_STATUS_NEW 1)
  (vanishes! HAS_CODE_NEW)
  (debug (eq! CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))))

(defun (dep_num_and_status_update_for_deployment_without_code_h)
 (begin
  (increment_dep_number_h)
  (vanishes! DEPLOYMENT_STATUS_NEW)
  (vanishes! CODE_SIZE_NEW)
  (vanishes HAS_CODE_NEW)
  (debug (eq! CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))))

;; Account inspection
(defun (account_opening_h)
 (begin
  (same_nonce_h)
  (same_code_h)
  (same_dep_num_and_status_h)))

(defun (account_viewing_h)
 (begin
  (account_opening_h)
  (same_balance_h)))

(defun (account_deletion_h)
 (begin
  (vanishes! NONCE_NEW)
  (vanishes! BALANCE_NEW)
  (vanishes! CODE_SIZE_NEW)
  (vanishes! HAS_CODE_NEW)
  (debug (eq! CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))
  fresh_new_dep_num_and_status_h))

(defun (same_addr_as_previously_v)
 (begin
  (remained-constant! ADDRESS_HI)
  (remained-constant! ADDRESS_LO)))

(defun (same_addr_and_dep_num_as_previously_v)
 (begin
  (same_addr_as_previously_v)
  (eq! DEPLOYMENT_NUMBER (prev DEPLOYMENT_NUMBER_NEW))))

(defun (same_addr_and_dep_num_and_dep_stage_as_previously_v)
 (begin
  (same_addr_and_dep_num_as_previously_v)
  (eq! DEPLOYMENT_STATUS (prev DEPLOYMENT_STATUS_NEW))))

(defun (deploy_empty_bytecode_h)
 (begin
  (eq! ADDRESS_HI CODE_ADDRESS_HI)
  (eq! ADDRESS_LO CODE_ADDRESS_LO)
  (eq! DEPLOYMENT_NUMBER CODE_DEPLOYMENT_NUMBER)
  (debug (eq! DEPLOYMENT_STATUS 1))
  (debug (vanishes! DEPLOYMENT_STATUS_NEW))
  (eq! DEPLOYMENT_STATUS_NEW (- DEPLOYMENT_STATUS 1))
  (vanishes! CODE_SIZE_NEW)
  (vanishes! HAS_CODE_NEW)
  (debug same_code_hash_h)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;  6.2 Code ownership   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint hascode_emptyness (:perspective account)
 (if-eq-else CODE_HASH_HI EMPTY_KECCAK_HI
    (if-eq-else CODE_HASH_LO EMPTY_KECCAK_LO
        (vanishes! HAS_CODE)
        (eq! HAS_CODE 1))
    (eq! HAS_CODE 1)))

(defconstraint hascode_new_emptyness (:perspective account)
               (if-eq-else    CODE_HASH_HI_NEW    EMPTY_KECCAK_HI
                              (if-eq-else    CODE_HASH_LO_NEW    EMPTY_KECCAK_LO
                                             (eq!    HAS_CODE_NEW 0)
                                             (eq!    HAS_CODE_NEW 1))
                              (eq!    HAS_CODE_NEW    1)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  6.3 Account existence   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; TODO DEBUG Only
(defconstraint exist_is_binary (:perspective account)
 (begin
  (is-binary EXISTS)
  (is-binary EXISTS_NEW)))

(defconstraint exists_is_on (:perspective account)
 (if-zero (+ (~ NONCE) (~ BALANCE) (~ HAS_CODE))
    (vanishes! EXISTS)
    (eq! EXISTS 1)))

(defconstraint exists_new_is_on (:perspective account)
 (if-zero (+ (~ NONCE_NEW) (~ BALANCE_NEW) (~ HAS_CODE_NEW))
    (vanishes! EXISTS_NEW)
    (eq! EXISTS_NEW 1)))
