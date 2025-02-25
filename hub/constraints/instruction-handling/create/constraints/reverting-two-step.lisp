(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;    X.Y.15 Two step reverting CREATE's   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---two-step-reverting-precondition)    (*    PEEK_AT_SCENARIO    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT))

(defconstraint    create-instruction---deployment-FAILURE_WILL_REVERT---fully-undoing-creator-account-operations
                  (:guard    (create-instruction---two-step-reverting-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-same-address-as                         CREATE_third_creator_account_row___row_offset    CREATE_first_creator_account_row___row_offset)
                    (account-same-balance                            CREATE_third_creator_account_row___row_offset)
                    (account-undo-nonce-update                       CREATE_third_creator_account_row___row_offset    CREATE_first_creator_account_row___row_offset)
                    (account-same-code                               CREATE_third_creator_account_row___row_offset)
                    (account-same-warmth                             CREATE_third_creator_account_row___row_offset)
                    (account-same-deployment-number-and-status       CREATE_third_creator_account_row___row_offset)
                    (account-same-marked-for-selfdestruct            CREATE_third_creator_account_row___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CREATE_third_creator_account_row___row_offset))
                    (vanishes!    (shift    account/ROMLEX_FLAG      CREATE_third_creator_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CREATE_third_creator_account_row___row_offset))
                    (DOM-SUB-stamps---revert-with-current            CREATE_third_creator_account_row___row_offset    2)
                    ))

(defconstraint    create-instruction---deployment-FAILURE_WILL_REVERT---fully-undoing-createe-account-operations
                  (:guard    (create-instruction---two-step-reverting-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (account-same-address-as                         CREATE_third_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-same-balance                            CREATE_third_createe_account_row___row_offset)
                    (account-same-nonce                              CREATE_third_createe_account_row___row_offset)
                    (account-same-code                               CREATE_third_createe_account_row___row_offset)
                    (account-undo-warmth-update                      CREATE_third_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-same-deployment-status                  CREATE_third_createe_account_row___row_offset)
                    (account-same-marked-for-selfdestruct            CREATE_third_createe_account_row___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CREATE_third_createe_account_row___row_offset))
                    (vanishes!    (shift    account/ROMLEX_FLAG      CREATE_third_createe_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CREATE_third_createe_account_row___row_offset))
                    (DOM-SUB-stamps---revert-with-current            CREATE_third_createe_account_row___row_offset    3)
                    ))
