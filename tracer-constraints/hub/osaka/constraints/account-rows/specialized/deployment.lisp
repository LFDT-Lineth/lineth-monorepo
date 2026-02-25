(module hub)


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

