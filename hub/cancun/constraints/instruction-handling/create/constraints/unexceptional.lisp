(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    X.Y.11 Exceptional CREATE's   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---unexceptional-CREATE-precondition)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CREATE---unexceptional)))

(defconstraint    create-instruction---creator-account-first-encouter    (:guard (create-instruction---unexceptional-CREATE-precondition))
                  (begin
                    (eq!          (shift    account/ADDRESS_HI       CREATE_first_creator_account_row___row_offset)    (create-instruction---creator-address-hi))
                    (eq!          (shift    account/ADDRESS_LO       CREATE_first_creator_account_row___row_offset)    (create-instruction---creator-address-lo))
                    ;; balance operation done below
                    ;; nonce   operation done below
                    (account-same-code                               CREATE_first_creator_account_row___row_offset)
                    (account-same-warmth                             CREATE_first_creator_account_row___row_offset)
                    (account-same-deployment-number-and-status       CREATE_first_creator_account_row___row_offset)
                    (account-same-marked-for-selfdestruct            CREATE_first_creator_account_row___row_offset)
                    (vanishes!    (shift    account/ROMLEX_FLAG      CREATE_first_creator_account_row___row_offset))
                    (vanishes!    (shift    account/TRM_FLAG         CREATE_first_creator_account_row___row_offset))
                    (eq!          (shift    account/RLPADDR_FLAG     CREATE_first_creator_account_row___row_offset)    (create-instruction---trigger_RLPADDR))
                    (DOM-SUB-stamps---standard                       CREATE_first_creator_account_row___row_offset
                                                                     0)
                    ))

(defconstraint    create-instruction---creator-balance-update            (:guard (create-instruction---unexceptional-CREATE-precondition))
                  (begin
                    (if-not-zero    (scenario-shorthand---CREATE---rebuffed)
                                    (account-same-balance    CREATE_first_creator_account_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed)
                                    (account-decrement-balance-by     CREATE_first_creator_account_row___row_offset
                                                                      (create-instruction---STACK-value-lo)))
                    ))

(defconstraint    create-instruction---creator-nonce-update              (:guard (create-instruction---unexceptional-CREATE-precondition))
                  (begin
                    (if-not-zero    (scenario-shorthand---CREATE---no-creator-state-change)
                                    (account-same-nonce         CREATE_first_creator_account_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CREATE---creator-state-change)
                                    (account-increment-nonce    CREATE_first_creator_account_row___row_offset))
                    ))

(defconstraint    create-instruction---creator-RLPADDR-parameters        (:guard (create-instruction---unexceptional-CREATE-precondition))
                  (if-not-zero    (shift    account/RLPADDR_FLAG     CREATE_first_creator_account_row___row_offset)
                                  (begin
                                    ;; (eq!    (shift    account/RLPADDR_FLAG         CREATE_first_creator_account_row___row_offset) )
                                    (eq!    (shift    account/RLPADDR_RECIPE       CREATE_first_creator_account_row___row_offset)    (create-instruction---address-computation-recipe))
                                    ;; (eq!    (shift    account/RLPADDR_DEP_ADDR_HI  CREATE_first_creator_account_row___row_offset) )
                                    ;; (eq!    (shift    account/RLPADDR_DEP_ADDR_LO  CREATE_first_creator_account_row___row_offset) )
                                    (eq!    (shift    account/RLPADDR_SALT_HI      CREATE_first_creator_account_row___row_offset)     (create-instruction---STACK-salt-hi))
                                    (eq!    (shift    account/RLPADDR_SALT_LO      CREATE_first_creator_account_row___row_offset)     (create-instruction---STACK-salt-lo))
                                    (eq!    (shift    account/RLPADDR_KEC_HI       CREATE_first_creator_account_row___row_offset)     (create-instruction---init-code-hash-hi))
                                    (eq!    (shift    account/RLPADDR_KEC_LO       CREATE_first_creator_account_row___row_offset)     (create-instruction---init-code-hash-lo))
                                    )))

(defun    (create-instruction---address-computation-recipe)    (+    (*   (create-instruction---is-CREATE)    RLP_ADDR_RECIPE_1)
                                                                     (*   (create-instruction---is-CREATE2)   RLP_ADDR_RECIPE_2)))

(defun    (create-instruction---init-code-hash-hi)    (*    (create-instruction---is-CREATE2)
                                                            (if-not-zero    (create-instruction---trigger_HASHINFO)
                                                                            ;; HASH_INFO required, nonempty code
                                                                            (create-instruction---HASHINFO-keccak-hi)
                                                                            ;; HASH_INFO not required, empty code
                                                                            EMPTY_KECCAK_HI)))

(defun    (create-instruction---init-code-hash-lo)    (*    (create-instruction---is-CREATE2)
                                                            (if-not-zero    (create-instruction---trigger_HASHINFO)
                                                                            ;; HASH_INFO needed to be triggered
                                                                            (create-instruction---HASHINFO-keccak-lo)
                                                                            ;; HASH_INFO didn't need to be triggered
                                                                            EMPTY_KECCAK_LO)))
