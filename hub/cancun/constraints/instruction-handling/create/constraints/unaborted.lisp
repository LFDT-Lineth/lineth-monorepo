(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;    X.Y.12 Unexceptional, unaborted CREATE's   ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (create-instruction---unexceptional-and-unaborted-precondition)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CREATE---compute-deployment-address)))

;; see specs for this suprising (and seemingly out of place) instance of address trimming ...
(defconstraint    create-instruction---createe-account-row-first-appearance    (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                  (begin
                    (eq!          (shift    account/ADDRESS_HI       CREATE_first_createe_account_row___row_offset)    (create-instruction---createe-address-hi))
                    (eq!          (shift    account/ADDRESS_LO       CREATE_first_createe_account_row___row_offset)    (create-instruction---createe-address-lo))
                    ;; balance           operation done below
                    ;; nonce             operation done below
                    ;; code              operation done below
                    ;; warmth            operation done below
                    ;; deployment number operation done below
                    ;; deployment status operation done below
                    (account-same-marked-for-deletion                CREATE_first_createe_account_row___row_offset)
                    (eq!          (shift    account/ROMLEX_FLAG      CREATE_first_createe_account_row___row_offset)    (create-instruction---trigger_ROMLEX))
                    (account-trim-address                            CREATE_first_createe_account_row___row_offset     ;; row offset
                                                                     (create-instruction---createe-address-hi)         ;; high part of raw, potentially untrimmed address
                                                                     (create-instruction---createe-address-lo))        ;; low  part of raw, potentially untrimmed address
                    (vanishes!    (shift    account/RLPADDR_FLAG     CREATE_first_createe_account_row___row_offset))
                    (DOM-SUB-stamps---standard                       CREATE_first_createe_account_row___row_offset
                                                                     1)
                    ))

(defconstraint    create-instruction---createe-balance-operation               (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                    (begin
                      (if-not-zero    (scenario-shorthand---CREATE---rebuffed)       (account-same-balance            CREATE_first_createe_account_row___row_offset))
                      (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed)   (account-increment-balance-by    CREATE_first_createe_account_row___row_offset
                                                                                                                  (create-instruction---STACK-value-lo)))
                      ))

(defconstraint    create-instruction---createe-nonce-operation                 (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                    (begin
                      (if-not-zero    (scenario-shorthand---CREATE---rebuffed)       (account-same-nonce         CREATE_first_createe_account_row___row_offset))
                      (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed)   (account-increment-nonce    CREATE_first_createe_account_row___row_offset))
                      ))

(defconstraint    create-instruction---createe-code-operation                  (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                  (begin
                    (if-not-zero    (scenario-shorthand---CREATE---rebuffed)       (account-same-code          CREATE_first_createe_account_row___row_offset))
                    (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed)   (begin
                                                                                 (account-same-code-hash   CREATE_first_createe_account_row___row_offset)
                                                                                 (eq!    (shift    account/CODE_SIZE_NEW    CREATE_first_createe_account_row___row_offset)
                                                                                         (create-instruction---STACK-size-lo))
                                                                                 (debug    (eq!    account/CODE_HASH_HI_NEW    EMPTY_KECCAK_HI))
                                                                                 (debug    (eq!    account/CODE_HASH_LO_NEW    EMPTY_KECCAK_LO))
                                                                                 ))
                    ))

(defconstraint    create-instruction---createe-warmth-operation                (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                    (begin
                      (if-not-zero    (scenario-shorthand---CREATE---no-creator-state-change)    (account-same-warmth       CREATE_first_createe_account_row___row_offset))
                      (if-not-zero    (scenario-shorthand---CREATE---creator-state-change)       (account-turn-on-warmth    CREATE_first_createe_account_row___row_offset))
                      ))

(defconstraint    create-instruction---createe-deployment-number-operation     (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                    (begin
                      (if-not-zero    (scenario-shorthand---CREATE---rebuffed)        (account-same-deployment-number         CREATE_first_createe_account_row___row_offset))
                      (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed)    (account-increment-deployment-number    CREATE_first_createe_account_row___row_offset))
                      ))

(defconstraint    create-instruction---createe-deployment-status-operation     (:guard    (create-instruction---unexceptional-and-unaborted-precondition))
                    (begin
                      (if-not-zero    (scenario-shorthand---CREATE---rebuffed)                           (account-same-deployment-status       CREATE_first_createe_account_row___row_offset))
                      (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)    (account-turn-on-deployment-status    CREATE_first_createe_account_row___row_offset))
                      (if-not-zero    (scenario-shorthand---CREATE---not-rebuffed-empty-init-code)       (account-same-deployment-status       CREATE_first_createe_account_row___row_offset))
                      ))
