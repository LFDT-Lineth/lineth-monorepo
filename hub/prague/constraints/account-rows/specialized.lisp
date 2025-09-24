(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;   X.1.1 Introduction                ;;
;;   X.1.2 Conventions                 ;;
;;   X.1.3 Account nonce constraints   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-nonce         relOffset)     (shift (eq! account/NONCE_NEW account/NONCE)       relOffset))
(defun (account-increment-nonce    relOffset)     (shift (eq! account/NONCE_NEW (+ account/NONCE 1)) relOffset))
(defun (account-undo-nonce-update  undoAt doneAt) (begin (eq! (shift account/NONCE_NEW undoAt) (shift account/NONCE      doneAt))
                                                         (eq! (shift account/NONCE     undoAt) (shift account/NONCE_NEW  doneAt))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   X.1.4 Account balance constraints   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-balance           relOffset)          (eq!       (shift    account/BALANCE_NEW    relOffset)         (shift account/BALANCE   relOffset)         ))         ;; relOffset rows into the future
(defun (account-increment-balance-by   relOffset value)    (eq!       (shift    account/BALANCE_NEW    relOffset)    (+   (shift account/BALANCE   relOffset)   value)))         ;; relOffset rows into the future
(defun (account-decrement-balance-by   relOffset value)    (eq!       (shift    account/BALANCE_NEW    relOffset)    (-   (shift account/BALANCE   relOffset)   value)))         ;; relOffset rows into the future
(defun (account-undo-balance-update    undoAt doneAt)      (begin
                                                             (eq!     (shift   account/BALANCE_NEW   undoAt)   (shift   account/BALANCE       doneAt))   ;; same as relOffset rows into the past
                                                             (eq!     (shift   account/BALANCE       undoAt)   (shift   account/BALANCE_NEW   doneAt))   ;; same as relOffset rows into the past
                                                             ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X.1.5 Account byte code constraints   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-code-size         relOffset)     (shift (eq! account/CODE_SIZE_NEW account/CODE_SIZE)              relOffset))
(defun (account-same-code-hash         relOffset)     (begin (shift (eq! account/CODE_HASH_HI_NEW account/CODE_HASH_HI) relOffset)
                                                             (shift (eq! account/CODE_HASH_LO_NEW account/CODE_HASH_LO) relOffset)))
(defun (account-same-code              relOffset)     (begin (account-same-code-size relOffset)
                                                             (account-same-code-hash relOffset)))

(defun (account-undo-code-size-update  undoAt doneAt)     (begin (eq! (shift account/CODE_SIZE_NEW      undoAt) (shift account/CODE_SIZE          doneAt))
                                                                 (eq! (shift account/CODE_SIZE          undoAt) (shift account/CODE_SIZE_NEW      doneAt))))
(defun (account-undo-code-hash-update  undoAt doneAt)     (begin (eq! (shift account/CODE_HASH_HI_NEW   undoAt) (shift account/CODE_HASH_HI       doneAt))
                                                                 (eq! (shift account/CODE_HASH_HI       undoAt) (shift account/CODE_HASH_HI_NEW   doneAt))
                                                                 (eq! (shift account/CODE_HASH_LO_NEW   undoAt) (shift account/CODE_HASH_LO       doneAt))
                                                                 (eq! (shift account/CODE_HASH_LO       undoAt) (shift account/CODE_HASH_LO_NEW   doneAt))))
(defun (account-undo-code-update       undoAt doneAt)     (begin (account-undo-code-size-update undoAt doneAt)
                                                                 (account-undo-code-hash-update undoAt doneAt)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;   X.1.6 Account warmth constraints   ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-warmth         kappa)          (shift (eq! account/WARMTH_NEW account/WARMTH) kappa))
(defun (account-turn-on-warmth      kappa)          (shift (eq! account/WARMTH_NEW 1             ) kappa))
(defun (account-undo-warmth-update  undoAt doneAt)  (begin (eq! (shift account/WARMTH_NEW undoAt) (shift account/WARMTH     doneAt))
                                                           (eq! (shift account/WARMTH     undoAt) (shift account/WARMTH_NEW doneAt))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;   X.1.7 Account deployment status constraints   ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-deployment-number             kappa)          (shift (eq! account/DEPLOYMENT_NUMBER_NEW account/DEPLOYMENT_NUMBER) kappa))
(defun (account-same-deployment-status             kappa)          (shift (eq! account/DEPLOYMENT_STATUS_NEW account/DEPLOYMENT_STATUS) kappa))
(defun (account-same-deployment-number-and-status  kappa)          (begin (account-same-deployment-number kappa)
                                                                          (account-same-deployment-status kappa)))
(defun (account-increment-deployment-number        kappa)          (shift (eq! account/DEPLOYMENT_NUMBER_NEW (+ 1 account/DEPLOYMENT_NUMBER)) kappa))
(defun (account-turn-on-deployment-status          kappa)          (shift (eq! account/DEPLOYMENT_STATUS_NEW (+ 1 account/DEPLOYMENT_STATUS)) kappa))
(defun (account-undo-deployment-status-update      undoAt doneAt)  (begin (account-same-deployment-number            undoAt)
                                                                          (eq! (shift account/DEPLOYMENT_STATUS_NEW  undoAt)  (shift account/DEPLOYMENT_STATUS     doneAt))
                                                                          (eq! (shift account/DEPLOYMENT_STATUS      undoAt)  (shift account/DEPLOYMENT_STATUS_NEW doneAt))))

;; not used in practice
(defun (account-initiate-for-deployment  relOffset init_code_size  value)
  (begin
    (debug (eq! (shift account/NONCE              relOffset) 0 ) )
    (eq!        (shift account/NONCE_NEW          relOffset) 1 )
    (account-increment-balance-by                 relOffset  value)
    (debug (eq! (shift account/CODE_SIZE          relOffset) 0 ) )
    (eq!        (shift account/CODE_SIZE_NEW      relOffset) init_code_size )
    (debug (eq! (shift account/HAS_CODE           relOffset) 0 ) )
    (debug (eq! (shift account/CODE_HASH_HI       relOffset) EMPTY_KECCAK_HI))
    (debug (eq! (shift account/CODE_HASH_LO       relOffset) EMPTY_KECCAK_LO))
    (eq!        (shift account/HAS_CODE_NEW       relOffset) 0 )
    (debug (eq! (shift account/CODE_HASH_HI_NEW   relOffset) EMPTY_KECCAK_HI))
    (debug (eq! (shift account/CODE_HASH_LO_NEW   relOffset) EMPTY_KECCAK_LO))
    (account-increment-deployment-number          relOffset)
    (account-turn-on-deployment-status            relOffset)
    (account-turn-on-warmth                       relOffset)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;;   X.1.8 Account MARKED_FOR_SELFDESTRUCT constraints   ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-same-marked-for-deletion  kappa) (shift (eq! account/MARKED_FOR_DELETION_NEW account/MARKED_FOR_DELETION) kappa))
(defun (account-mark-account-for-deletion kappa) (shift (eq! account/MARKED_FOR_DELETION_NEW 1                              ) kappa))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   X.1.9 Account inspection constraints   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-opening kappa) (begin (account-same-nonce                        kappa)
                                      (account-same-code                         kappa)
                                      (account-same-deployment-number-and-status kappa)))

(defun (account-viewing kappa) (begin (account-opening      kappa)
                                      (account-same-balance kappa)))

;; never used in practice
(defun (account-deletion kappa) (begin (vanishes!        (shift account/NONCE_NEW           kappa))
                                       (vanishes!        (shift account/BALANCE_NEW         kappa))
                                       (vanishes!        (shift account/CODE_SIZE_NEW       kappa))
                                       (vanishes!        (shift account/HAS_CODE_NEW        kappa))
                                       (debug     (eq!   (shift account/CODE_HASH_HI_NEW    kappa)   EMPTY_KECCAK_HI))
                                       (debug     (eq!   (shift account/CODE_HASH_LO_NEW    kappa)   EMPTY_KECCAK_LO))
                                       (account-fresh-new-deployment-number-and-status      kappa)))

(defun (account-fresh-new-deployment-number-and-status    kappa) (begin  (account-increment-deployment-number                        kappa)
                                                                         (vanishes!             (shift account/DEPLOYMENT_STATUS_NEW kappa))
                                                                         (debug      (vanishes! (shift account/DEPLOYMENT_STATUS     kappa)))))


(defun (account-same-address-as  undoAt
                                 doneAt)
  (begin (eq! (shift account/ADDRESS_HI undoAt) (shift account/ADDRESS_HI doneAt))   ;; action performed doneAt many rows from here
         (eq! (shift account/ADDRESS_LO undoAt) (shift account/ADDRESS_LO doneAt)))) ;; action undone    undoAt many rows from here

(defun (account-same-address-and-deployment-number-as undoAt doneAt) (begin (account-same-address-as undoAt doneAt)
                                                                            (eq! (shift account/DEPLOYMENT_NUMBER undoAt) (shift account/DEPLOYMENT_NUMBER_NEW doneAt))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   X.1.10 Code fragment index and trimming   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-retrieve-code-fragment-index    kappa) (eq! (shift account/ROMLEX_FLAG   kappa) 1))

(defun (account-trim-address   kappa                   ;; row offset
                               raw-address-hi          ;; high part of raw, potentially untrimmed address
                               raw-address-lo          ;; low  part of raw, potentially untrimmed address
                               ) (begin
                               (eq! (shift   account/TRM_FLAG             kappa) 1)
                               (eq! (shift   account/TRM_RAW_ADDRESS_HI   kappa) raw-address-hi)
                               (eq! (shift   account/ADDRESS_LO           kappa) raw-address-lo)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   X.1.11 Precompile flag related   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (account-isnt-precompile   kappa) (vanishes!   (shift    account/IS_PRECOMPILE    kappa)))
(defun (account-is-precompile     kappa) (eq!         (shift    account/IS_PRECOMPILE    kappa) 1))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   X.2 Code ownership constraints   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   account---code-ownership-curr (:perspective account)
                 (if-eq-else account/CODE_HASH_HI EMPTY_KECCAK_HI
                             (if-eq-else account/CODE_HASH_LO EMPTY_KECCAK_LO
                                         (eq! account/HAS_CODE 0)
                                         (eq! account/HAS_CODE 1))
                             (eq! account/HAS_CODE 1)))

(defconstraint   account---code-ownership-next (:perspective account)
                 (if-eq-else account/CODE_HASH_HI_NEW EMPTY_KECCAK_HI
                             (if-eq-else account/CODE_HASH_LO_NEW EMPTY_KECCAK_LO
                                         (eq! account/HAS_CODE_NEW 0)
                                         (eq! account/HAS_CODE_NEW 1))
                             (eq! account/HAS_CODE_NEW 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   X.3 Account existence constraints   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   account---existence-curr (:perspective account)
                 (begin (debug (is-binary account/EXISTS))
                        (if-not-zero account/NONCE       (eq! account/EXISTS 1))
                        (if-not-zero account/BALANCE     (eq! account/EXISTS 1))
                        (if-not-zero account/HAS_CODE    (eq! account/EXISTS 1))
                        (if-zero account/NONCE
                                 (if-zero account/BALANCE
                                          (if-zero account/HAS_CODE (eq! account/EXISTS 0))))))

(defconstraint   account---existence-next (:perspective account)
                 (begin (debug (is-binary account/EXISTS_NEW))
                        (if-not-zero account/NONCE_NEW    (eq! account/EXISTS_NEW 1))
                        (if-not-zero account/BALANCE_NEW  (eq! account/EXISTS_NEW 1))
                        (if-not-zero account/HAS_CODE_NEW (eq! account/EXISTS_NEW 1))
                        (if-zero account/NONCE_NEW
                                 (if-zero account/BALANCE_NEW
                                          (if-zero account/HAS_CODE_NEW (eq! account/EXISTS_NEW 0))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;   X.4 Generalities on ROMLEX_FLAG   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   account---the-ROMLEX-lookup-requires-nonzero-code-size (:perspective account)
                 (if-zero    account/CODE_SIZE_NEW
                             (vanishes!    account/ROMLEX_FLAG)))
