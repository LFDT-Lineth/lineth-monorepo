(module hub)


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

