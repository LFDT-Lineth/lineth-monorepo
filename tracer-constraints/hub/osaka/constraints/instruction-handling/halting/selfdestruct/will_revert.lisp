(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.5 SELFDESTRUCT   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                               ;;
;;    X.5.6 Undoing rows for scenario/SELFDESTRUCT_WILL_REVERT   ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (selfdestruct-instruction---scenario-WILL_REVERT-precondition)    (*    PEEK_AT_SCENARIO    scenario/SELFDESTRUCT_WILL_REVERT))

(defconstraint    selfdestruct-instruction---first-undoing-row-for-WILL_REVERT-scenario
                  (:guard (selfdestruct-instruction---scenario-WILL_REVERT-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (debug (eq! (shift account/ROMLEX_FLAG       ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW) 0))
                    (debug (eq! (shift account/TRM_FLAG          ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW) 0))
                    (account-same-address-as                     ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                    (account-undo-balance-update                 ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                    (account-undo-nonce-update                   ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                    (account-undo-warmth-update                  ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                    (account-undo-code-update                    ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                    (account-undo-deployment-status-update       ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW)
                    (account-same-marked-for-deletion            ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW)
                    (DOM-SUB-stamps---revert-with-current        ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW 2)))

(defconstraint    selfdestruct-instruction---second-undoing-row-for-WILL_REVERT-scenario
                  (:guard (selfdestruct-instruction---scenario-WILL_REVERT-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (debug (eq! (shift account/ROMLEX_FLAG       ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW) 0))
                    (debug (eq! (shift account/TRM_FLAG          ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW) 0))
                    (account-same-address-as                     ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                    (account-undo-balance-update                 ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                    (account-undo-nonce-update                   ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                    (account-undo-warmth-update                  ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                    (account-undo-code-update                    ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                    (account-undo-deployment-status-update       ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW)
                    (account-same-marked-for-deletion            ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW)
                    (DOM-SUB-stamps---revert-with-current        ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW 3)))
