(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   X.Y.Z Delegation constraints   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   account---delegation---setting-is-delegated
                 (:perspective   account)
                 ;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   DELEGATION_ADDRESS_HI   (eq!   IS_DELEGATED   1))
                   (if-not-zero   DELEGATION_ADDRESS_LO   (eq!   IS_DELEGATED   1))
                   (if-zero       DELEGATION_ADDRESS_HI
                                  (if-zero   DELEGATION_ADDRESS_LO  (eq!   IS_DELEGATED   0)))
                   ))


(defconstraint   account---delegation---setting-is-delegated-new
                 (:perspective   account)
                 ;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   DELEGATION_ADDRESS_HI_NEW   (eq!   IS_DELEGATED_NEW   1))
                   (if-not-zero   DELEGATION_ADDRESS_LO_NEW   (eq!   IS_DELEGATED_NEW   1))
                   (if-zero       DELEGATION_ADDRESS_HI_NEW
                                  (if-zero   DELEGATION_ADDRESS_LO_NEW  (eq!   IS_DELEGATED_NEW   0)))
                   ))

(defconstraint   account---delegation---delegation-information-may-only-change-during-the-TX_AUTH-phase
                 (:perspective   account)
                 ;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   TX_AUTH
                            (begin   (account-same-delegation-address   0)
                                     (account-same-delegation-number    0)
                                     (account-same-delegation-status    0)
                                     )))

(defconstraint   account---delegation---delegated-accounts-have-code-and-known-code-size
                 (:perspective   account)
                 ;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   IS_DELEGATED
                                  (begin
                                    (eq!  CODE_SIZE  EOA_DELEGATED_CODE_LENGTH )
                                    (eq!  HAS_CODE   1                         )
                                    ))
                   (if-not-zero   IS_DELEGATED_NEW
                                  (begin
                                    (eq!  CODE_SIZE_NEW  EOA_DELEGATED_CODE_LENGTH )
                                    (eq!  HAS_CODE_NEW   1                         )
                                    ))
                   ))

(defconstraint   account---delegation---accounts-with-empty-code-may-not-check-for-delegation
                 (:perspective   account)
                 ;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-zero   HAS_CODE       (vanishes!   CHECK_FOR_DELEGATION     ))
                   (if-zero   HAS_CODE_NEW   (vanishes!   CHECK_FOR_DELEGATION_NEW ))
                   ))

(defconstraint   account---delegation---checking-for-delegation-for-new-address-shouldnt-take-place-outside-of-TX_AUTH-rows
                 (:perspective   account)
                 ;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-zero   TX_AUTH
                            (vanishes!   CHECK_FOR_DELEGATION_NEW ))
                 )
