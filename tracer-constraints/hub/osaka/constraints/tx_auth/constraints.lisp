(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;   X Authorization phase     ;;
;;   X.Y Constraints           ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF___ACCOUNT_DELEGATION___AUTH_ROW  -1
  ROFF___ACCOUNT_DELEGATION___ACC_ROW    0
  )

(defun   (tx-auth---AUTH---authority-address-hi)  (shift  auth/AUTHORITY_ADDRESS_HI  ROFF___ACCOUNT_DELEGATION___AUTH_ROW))
(defun   (tx-auth---AUTH---authority-address-lo)  (shift  auth/AUTHORITY_ADDRESS_LO  ROFF___ACCOUNT_DELEGATION___AUTH_ROW))



(defproperty     authorization-phase---account-delegation-constraints---sanity-checks
                 (if-not-zero  (TX_AUTH-phase-account-row)
                               (begin
                                 (eq!  (shift TX_AUTH                           ROFF___ACCOUNT_DELEGATION___AUTH_ROW )  1)
                                 (eq!  (shift PEEK_AT_AUTHORIZATION             ROFF___ACCOUNT_DELEGATION___AUTH_ROW )  1)
                                 (eq!  (shift auth/AUTHORITY_ECRECOVER_SUCCESS  ROFF___ACCOUNT_DELEGATION___AUTH_ROW )  1)
                                 )))

(defconstraint   authorization-phase---account-delegation-constraints---peeking-into-authority            (:guard (TX_AUTH-phase-account-row))
                 (begin
                   (eq!  (shift  account/ADDRESS_HI  ROFF___ACCOUNT_DELEGATION___ACC_ROW)  (tx-auth---AUTH---authority-address-hi))
                   (eq!  (shift  account/ADDRESS_LO  ROFF___ACCOUNT_DELEGATION___ACC_ROW)  (tx-auth---AUTH---authority-address-lo))
                   (account-trim-address             ROFF___ACCOUNT_DELEGATION___ACC_ROW        ;; row offset
                                                     (tx-auth---AUTH---authority-address-hi)            ;; high part of raw, potentially untrimmed address
                                                     (tx-auth---AUTH---authority-address-lo))           ;; low  part of raw, potentially untrimmed address
                   (account-same-balance                          ROFF___ACCOUNT_DELEGATION___ACC_ROW)
                   ;; nonce update
                   ;; code hash update
                   ;; code size update
                   ;; delegation check
                   ;; delegation address update
                   ;; delegation number update
                   ;; delegation status update
                   (account-same-deployment-number-and-status     ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                   (account-turn-on-warmth                        ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                   (account-same-marked-for-deletion              ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                   (DOM-SUB-stamps---standard                     ROFF___ACCOUNT_DELEGATION___ACC_ROW
                                                                  ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                   ))

(defconstraint   authorization-phase---account-delegation-constraints---triggering-delegation-detection
                 (:guard (TX_AUTH-phase-account-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (account-check-for-delegation-in-authorization-phase  ROFF___ACCOUNT_DELEGATION___ACC_ROW)
                 )

(defun  (valid-tuple)       (shift  auth/AUTHORIZATION_TUPLE_IS_VALID  ROFF___ACCOUNT_DELEGATION___AUTH_ROW))
(defun  (delegation-reset)  (shift  auth/DELEGATION_ADDRESS_IS_ZERO    ROFF___ACCOUNT_DELEGATION___AUTH_ROW))

(defun  (invalid-tuple)      (-  1  (valid-tuple)))
(defun  (proper-delegation)  (-  1  (delegation-reset)))

(defun  (check-post-delegation)  (*  (valid-tuple)
                                     (proper-delegation)))


;;--------------------------------------;;
;;   Invalid authorization tuple case   ;;
;;     ==> no-op                        ;;
;;--------------------------------------;;


(defconstraint   authorization-phase---account-delegation-constraints---invalid-authorization-tuple---no-updates
                 (:guard   (TX_AUTH-phase-account-row))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero  (invalid-tuple)
                               (begin
                                 (account-same-nonce                ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                                 (account-same-code-hash            ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                                 (account-same-code-size            ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                                 (account-same-delegation-address   ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                                 (account-same-delegation-number    ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                                 (account-same-delegation-status    ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                                 )))



;;------------------------------------;;
;;   Valid authorization tuple case   ;;
;;     ==> various updates            ;;
;;------------------------------------;;


(defun   (perform-delegation-operation)   (*  (TX_AUTH-phase-account-row)  (valid-tuple)))


(defconstraint   authorization-phase---account-delegation-constraints---valid-authorization-tuple---common-part
                 (:guard   (perform-delegation-operation))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (account-increment-nonce              ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                   (account-increment-delegation-number  ROFF___ACCOUNT_DELEGATION___ACC_ROW )
                   ))

(defconstraint   authorization-phase---account-delegation-constraints---valid-authorization-tuple---code-hash-update
                 (:guard   (perform-delegation-operation))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (delegation-reset)
                                (begin
                                  (eq!   (shift account/HAS_CODE_NEW       ROFF___ACCOUNT_DELEGATION___ACC_ROW ) 0 ) ;; extraneous, sanity check
                                  (eq!   (shift account/CODE_HASH_HI_NEW   ROFF___ACCOUNT_DELEGATION___ACC_ROW ) EMPTY_KECCAK_HI )
                                  (eq!   (shift account/CODE_HASH_LO_NEW   ROFF___ACCOUNT_DELEGATION___ACC_ROW ) EMPTY_KECCAK_LO )
                                  )))

(defconstraint   authorization-phase---account-delegation-constraints---valid-authorization-tuple---code-size-update
                 (:guard   (perform-delegation-operation))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   (delegation-reset)
                                  (eq!   (shift account/CODE_SIZE_NEW   ROFF___ACCOUNT_DELEGATION___ACC_ROW ) 0 ))
                   (if-not-zero   (proper-delegation)
                                  (eq!   (shift account/CODE_SIZE_NEW   ROFF___ACCOUNT_DELEGATION___ACC_ROW ) EOA_DELEGATED_CODE_LENGTH ))
                   ))

(defconstraint   authorization-phase---account-delegation-constraints---valid-authorization-tuple---delegation-address-update
                 (:guard   (perform-delegation-operation))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (account-set-delegation-address   ROFF___ACCOUNT_DELEGATION___ACC_ROW
                                                   (shift   auth/DELEGATION_ADDRESS_HI   ROFF___ACCOUNT_DELEGATION___AUTH_ROW )
                                                   (shift   auth/DELEGATION_ADDRESS_LO   ROFF___ACCOUNT_DELEGATION___AUTH_ROW )
                                                   ))

(defconstraint   authorization-phase---account-delegation-constraints---valid-authorization-tuple---delegation-status-update
                 (:guard   (perform-delegation-operation))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (shift   account/IS_DELEGATED_NEW   ROFF___ACCOUNT_DELEGATION___ACC_ROW)   (proper-delegation))
                 )

;; (defconstraint   authorization-phase---account-delegation-constraints---  (:guard  (TX_AUTH-phase-account-row))
