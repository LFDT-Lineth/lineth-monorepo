(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.5 SELFDESTRUCT   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                                      ;;
;;    X.5.6 Account deletion row for scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED   ;;
;;                                                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition)    (*    PEEK_AT_SCENARIO    scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED))




(defconstraint    selfdestruct-instruction---account-deletion-row---flags
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (debug (eq! (shift account/ROMLEX_FLAG           ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) 0))
                    (debug (eq! (shift account/TRM_FLAG              ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) 0))))

(defconstraint    selfdestruct-instruction---account-deletion-row---address-duplication
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (account-same-address-as                         ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))

(defconstraint    selfdestruct-instruction---account-deletion-row---balance-squashing
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!        (shift account/BALANCE_NEW           ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) 0))

(defconstraint    selfdestruct-instruction---account-deletion-row---nonce-squashing
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!        (shift account/NONCE_NEW             ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) 0))

(defconstraint    selfdestruct-instruction---account-deletion-row---no-change-in-warmth
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (account-same-warmth                             ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW))

(defconstraint    selfdestruct-instruction---account-deletion-row---code-squashing
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!        (shift account/CODE_SIZE_NEW         ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) 0)
                    (eq!        (shift account/CODE_HASH_HI_NEW      ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) EMPTY_KECCAK_HI)
                    (eq!        (shift account/CODE_HASH_LO_NEW      ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW) EMPTY_KECCAK_LO)))

(defconstraint    selfdestruct-instruction---account-deletion-row---fresh-new-deployment
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (shift      (eq!   account/DEPLOYMENT_NUMBER_NEW (+ 1 account/DEPLOYMENT_NUMBER))                   ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW)
                    (shift      (eq!   account/DEPLOYMENT_STATUS_NEW 0                              )                   ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW)))

(defconstraint    selfdestruct-instruction---account-deletion-row---no-change-in-MARKED_FOR_SELFDESTRUCT
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (account-same-marked-for-deletion                ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW))

(defconstraint    selfdestruct-instruction---account-deletion-row---DOM_SUB-stamps
                  (:guard (selfdestruct-instruction---scenario-WONT_REVERT_NOT_YET_MARKED-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (selfdestruct-dom-sub-stamps                     ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW))
