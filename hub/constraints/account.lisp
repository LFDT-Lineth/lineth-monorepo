(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;  6.1 Specialized-rows ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Nonce constraintes

(defun (same_nonce_h)
 (eq! account/NONCE account/NONCE_NEW))

(defun (increment_nonce_h)
 (eq! account/NONCE_NEW (+ account/NONCE 1)))

(defun (decrement_nonce_h)
 (eq! account/NONCE_NEW (- account/NONCE 1)))

(defun (undo_account_nonce_update_v)
 (begin
  (eq! account/NONCE  (- (prev account/NONCE_NEW) 1))
  (eq! account/NONCE_NEW (prev account/NONCE))))

(defun (undo_previous_account_nonce_update_v)
 (begin
  (eq! account/NONCE  (- (shift account/NONCE_NEW -2) 1))
  (eq! account/NONCE_NEW (shift account/NONCE     -2))))

;; Balance constraints
(defun (same_balance_h)
 (eq! account/BALANCE_NEW account/BALANCE))

(defun (undo_account_balance_update_v)
 (begin
  (eq! account/BALANCE (prev account/BALANCE_NEW))
  (eq! account/BALANCE_NEW (prev account/BALANCE))))

(defun (undo_previous_account_balance_update_v)
 (begin
  (eq! account/BALANCE     (shift account/BALANCE_NEW -2))
  (eq! account/BALANCE_NEW (shift account/BALANCE     -2))))

;; Warmth constraints
(defun (same_account_warmth_h)
 (eq! account/WARMTH_NEW account/WARMTH))

(defun (turn_on_account_warmth_h)
 (eq! account/WARMTH_NEW 1))

(defun (undo_account_warmth_update_v)
 (begin
  (eq! account/WARMTH     (prev account/WARMTH_NEW))
  (eq! account/WARMTH_NEW (prev account/WARMTH))))

;; Code constraints
(defun (same_code_size_h)
 (eq! account/CODE_SIZE_NEW account/CODE_SIZE))

(defun (same_code_hash_h)
 (begin
  (eq! account/CODE_HASH_HI_NEW account/CODE_HASH_HI)
  (eq! account/CODE_HASH_LO_NEW account/CODE_HASH_LO)))

(defun (same_code_h)
 (begin
  (same_code_size_h)
  (same_code_hash_h)))

(defun (undo_code_size_update_v)
 (begin
  (eq! account/CODE_SIZE     (prev account/CODE_SIZE_NEW))
  (eq! account/CODE_SIZE_NEW (prev account/CODE_SIZE))))

(defun (undo_code_hash_update_v)
 (begin
  (eq! account/CODE_HASH_HI     (prev account/CODE_HASH_HI_NEW))
  (eq! account/CODE_HASH_HI_NEW (prev account/CODE_HASH_HI))
  (eq! account/CODE_HASH_LO     (prev account/CODE_HASH_LO_NEW))
  (eq! account/CODE_HASH_LO_NEW (prev account/CODE_HASH_LO))))

;; Deployment status constraints
(defun (same_dep_number_h)
 (eq! account/DEPLOYMENT_NUMBER_NEW account/DEPLOYMENT_NUMBER))

(defun (same_dep_status_h)
 (eq! account/DEPLOYMENT_STATUS_NEW account/DEPLOYMENT_STATUS))

(defun (same_dep_num_and_status_h)
 (begin
  (same_dep_number_h)
  (same_dep_status_h)))

(defun (undo_dep_number_update_v)
 (begin
  (eq! account/DEPLOYMENT_NUMBER     (prev account/DEPLOYMENT_NUMBER_NEW))
  (eq! account/DEPLOYMENT_NUMBER_NEW (prev account/DEPLOYMENT_NUMBER))))

(defun (undo_dep_status_update_v)
 (begin
  (eq! account/DEPLOYMENT_STATUS     (prev account/DEPLOYMENT_STATUS_NEW))
  (eq! account/DEPLOYMENT_STATUS_NEW (prev account/DEPLOYMENT_STATUS))))

(defun (undo_dep_status_and_number_update_v)
 (begin
  (undo_dep_number_update_v)
  (undo_dep_status_update_v)))

(defun (increment_dep_number_h)
 (eq! account/DEPLOYMENT_NUMBER_NEW (+ account/DEPLOYMENT_NUMBER 1)))

(defun (fresh_new_dep_num_and_status_h)
 (begin
  (vanishes! account/NONCE_NEW)
  (vanishes! account/CODE_SIZE_NEW)
  (vanishes! account/HAS_CODE_NEW)
  (debug (eq! account/CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! account/CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))
  (increment_dep_number_h)
  (vanishes! account/DEPLOYMENT_STATUS_NEW)))

(defun (dep_num_and_status_update_for_deployment_with_code_h)
 (begin
  (increment_dep_number_h)
  (eq! account/DEPLOYMENT_STATUS_NEW 1)
  (vanishes! account/HAS_CODE_NEW)
  (debug (eq! account/CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! account/CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))))

(defun (dep_num_and_status_update_for_deployment_without_code_h)
 (begin
  (increment_dep_number_h)
  (vanishes! account/DEPLOYMENT_STATUS_NEW)
  (vanishes! account/CODE_SIZE_NEW)
  (vanishes! account/HAS_CODE_NEW)
  (debug (eq! account/CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! account/CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))))

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
  (vanishes! account/NONCE_NEW)
  (vanishes! account/BALANCE_NEW)
  (vanishes! account/CODE_SIZE_NEW)
  (vanishes! account/HAS_CODE_NEW)
  (debug (eq! account/CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
  (debug (eq! account/CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))
  (fresh_new_dep_num_and_status_h)))

(defun (same_addr_as_previously_v)
 (begin
  (remained-constant! account/ADDRESS_HI)
  (remained-constant! account/ADDRESS_LO)))

(defun (same_addr_and_dep_num_as_previously_v)
 (begin
  (same_addr_as_previously_v)
  (eq! account/DEPLOYMENT_NUMBER (prev account/DEPLOYMENT_NUMBER_NEW))))

(defun (same_addr_and_dep_num_and_dep_stage_as_previously_v)
 (begin
  (same_addr_and_dep_num_as_previously_v)
  (eq! account/DEPLOYMENT_STATUS (prev account/DEPLOYMENT_STATUS_NEW))))

;; (defun (deploy_empty_bytecode_h)
;;  (begin
;;   (eq! account/ADDRESS_HI CODE_ADDRESS_HI)
;;   (eq! account/ADDRESS_LO CODE_ADDRESS_LO)
;;   (eq! account/DEPLOYMENT_NUMBER CODE_DEPLOYMENT_NUMBER)
;;   (debug (eq! account/DEPLOYMENT_STATUS 1))
;;   (debug (vanishes! account/DEPLOYMENT_STATUS_NEW))
;;   (eq! account/DEPLOYMENT_STATUS_NEW (- account/DEPLOYMENT_STATUS 1))
;;   (vanishes! account/CODE_SIZE_NEW)
;;   (vanishes! account/HAS_CODE_NEW)
;;   (debug same_code_hash_h)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;  6.2 Code ownership   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint hascode_emptyness (:perspective account)
 (if-eq-else account/CODE_HASH_HI EMPTY_KECCAK_HI
    (if-eq-else account/CODE_HASH_LO EMPTY_KECCAK_LO
        (vanishes! account/HAS_CODE)
        (eq!       account/HAS_CODE 1))
    (eq! account/HAS_CODE 1)))

(defconstraint hascode_new_emptyness (:perspective account)
               (if-eq-else    account/CODE_HASH_HI_NEW    EMPTY_KECCAK_HI
                              (if-eq-else    account/CODE_HASH_LO_NEW    EMPTY_KECCAK_LO
                                             (eq!    account/HAS_CODE_NEW 0)
                                             (eq!    account/HAS_CODE_NEW 1))
                              (eq!    account/HAS_CODE_NEW    1)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  6.3 Account existence   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; TODO DEBUG Only
(defconstraint exist_is_binary (:perspective account)
 (begin
  (is-binary account/EXISTS)
  (is-binary account/EXISTS_NEW)))

(defconstraint exists_is_on (:perspective account)
 (if-zero (+ (~ account/NONCE) (~ account/BALANCE) (~ account/HAS_CODE))
    (vanishes! account/EXISTS)
    (eq!       account/EXISTS 1)))

(defconstraint exists_new_is_on (:perspective account)
 (if-zero (+ (~ account/NONCE_NEW) (~ account/BALANCE_NEW) (~ account/HAS_CODE_NEW))
    (vanishes! account/EXISTS_NEW)
    (eq!       account/EXISTS_NEW 1)))
