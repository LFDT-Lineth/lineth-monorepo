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
;;    X.Y.13 One step reverting CREATE's   ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---one-step-reverting-precondition)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---CREATE---simple-revert)))

(defconstraint    create-instruction---one-step-reverting---undoing-creator-account-operations    (:guard    (create-instruction---one-step-reverting-precondition))
                  (begin
                    (account-same-address-as                         CREATE_second_creator_account_row___row_offset    CREATE_first_creator_account_row___row_offset)
                    (account-undo-balance-update                     CREATE_second_creator_account_row___row_offset    CREATE_first_creator_account_row___row_offset)
                    (account-undo-nonce-update                       CREATE_second_creator_account_row___row_offset    CREATE_first_creator_account_row___row_offset)
                    (account-same-code                               CREATE_second_creator_account_row___row_offset)
                    (account-same-warmth                             CREATE_second_creator_account_row___row_offset)
                    (account-same-deployment-number-and-status       CREATE_second_creator_account_row___row_offset)
                    (account-same-marked-for-selfdestruct            CREATE_second_creator_account_row___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CREATE_second_creator_account_row___row_offset))
                    (vanishes!    (shift    account/ROMLEX_FLAG      CREATE_second_creator_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CREATE_second_creator_account_row___row_offset))    ;; TODO: these 3 bit vanishing constraints could be merged
                    (DOM-SUB-stamps---revert-with-current            CREATE_second_creator_account_row___row_offset    0)
                    ))

(defconstraint    create-instruction---one-step-reverting---undoing-createe-account-operations    (:guard    (create-instruction---one-step-reverting-precondition))
                  (begin
                    (account-same-address-as                         CREATE_second_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-undo-balance-update                     CREATE_second_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-undo-nonce-update                       CREATE_second_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-undo-code-update                        CREATE_second_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-undo-warmth-update                      CREATE_second_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-undo-deployment-status-update           CREATE_second_createe_account_row___row_offset    CREATE_first_createe_account_row___row_offset)
                    (account-same-marked-for-selfdestruct            CREATE_second_createe_account_row___row_offset)
                    (vanishes!    (shift    account/TRM_FLAG         CREATE_second_createe_account_row___row_offset))
                    (vanishes!    (shift    account/ROMLEX_FLAG      CREATE_second_createe_account_row___row_offset))
                    (vanishes!    (shift    account/RLPADDR_FLAG     CREATE_second_createe_account_row___row_offset))    ;; TODO: these 3 bit vanishing constraints could be merged
                    (DOM-SUB-stamps---revert-with-current            CREATE_second_createe_account_row___row_offset    1)
                    ))
