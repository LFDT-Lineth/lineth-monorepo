(module hub)

;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;
;;;;               ;;;;
;;;;    X.Y STOP   ;;;;
;;;;               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    X.Y.1 Introduction   ;;
;;    X.Y.2 Shorthands     ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (stop-instruction---during-active-deployment)        (next context/BYTE_CODE_DEPLOYMENT_STATUS))
(defun   (stop-instruction---code-address-hi)   (next context/BYTE_CODE_ADDRESS_HI))
(defun   (stop-instruction---code-address-lo)   (next context/BYTE_CODE_ADDRESS_LO))
(defun   (stop-instruction---will-revert)       CONTEXT_WILL_REVERT)

(defun (stop-instruction---standard-precondition) (* PEEK_AT_STACK
                                                     stack/HALT_FLAG
                                                     (halting-instruction---is-STOP))) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.5.1 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   stop-instruction---stack-pattern (:guard (stop-instruction---standard-precondition))
                 (stack-pattern-0-0))

(defconstraint   stop-instruction---gas-cost (:guard (stop-instruction---standard-precondition))
                 (vanishes! GAS_COST))

(defconstraint   stop-instruction---raises-no-exceptions (:guard (stop-instruction---standard-precondition))
                 (vanishes! XAHOY))

(defconstraint   stop-instruction---first-non-stack-row (:guard (stop-instruction---standard-precondition))
                 (begin
                   (will-eq!          PEEK_AT_CONTEXT 1)
                   (read-context-data 1 CONTEXT_NUMBER)))

(defconstraint   stop-instruction---setting-NSR-and-peeking-flags (:guard (stop-instruction---standard-precondition))
                 (begin
                   (eq! NSR
                        (+ 2
                           (* (stop-instruction---during-active-deployment)
                              (+ 1 (stop-instruction---will-revert)))))
                   (if-zero (force-bin (stop-instruction---during-active-deployment))
                            (begin
                              (debug (eq! NSR 2) )
                              (eq! NSR
                                   (+ (shift PEEK_AT_CONTEXT 1)
                                      (shift PEEK_AT_CONTEXT 2)))
                              (execution-provides-empty-return-data 2))
                            (if-zero (force-bin (stop-instruction---will-revert))
                                     (begin
                                       (debug (eq! NSR 3) )
                                       (eq! NSR
                                            (+ (shift PEEK_AT_CONTEXT 1)
                                               (shift PEEK_AT_ACCOUNT 2)
                                               (shift PEEK_AT_CONTEXT 3)))
                                       (execution-provides-empty-return-data 3))
                                     (begin
                                       (debug (eq! NSR 4) )
                                       (eq! NSR
                                            (+ (shift PEEK_AT_CONTEXT 1)
                                               (shift PEEK_AT_ACCOUNT 2)
                                               (shift PEEK_AT_ACCOUNT 3)
                                               (shift PEEK_AT_CONTEXT 4)))
                                       (execution-provides-empty-return-data 4))))))


(defconstraint   stop-instruction---first-address-row (:guard (*    (stop-instruction---standard-precondition)    (stop-instruction---during-active-deployment)))
                 (begin (debug (vanishes!       (shift account/TRM_FLAG     2)))
                        (debug (vanishes!       (shift account/ROMLEX_FLAG  2)))
                        (eq! (shift account/ADDRESS_HI  2) (stop-instruction---code-address-hi))
                        (eq! (shift account/ADDRESS_LO  2) (stop-instruction---code-address-lo))
                        (account-same-balance    2)
                        (account-same-nonce      2)
                        (account-same-warmth     2)
                        (vanishes! (shift account/CODE_SIZE_NEW    2))
                        (eq!       (shift account/CODE_HASH_HI_NEW 2) EMPTY_KECCAK_HI)
                        (eq!       (shift account/CODE_HASH_LO_NEW 2) EMPTY_KECCAK_LO)
                        (account-same-deployment-number 2)
                        (eq!       (shift account/DEPLOYMENT_STATUS     2) 1)
                        (eq!       (shift account/DEPLOYMENT_STATUS_NEW 2) 0)
                        (DOM-SUB-stamps---standard    2
                                                      0)))

(defconstraint   stop-instruction---second-address-row (:guard (*    (stop-instruction---standard-precondition)    (stop-instruction---during-active-deployment)    (stop-instruction---will-revert)))
                 (begin (debug (vanishes! (shift account/TRM_FLAG     3)))
                        (debug (vanishes! (shift account/ROMLEX_FLAG  3)))
                        (account-same-address-as               3 2)
                        (account-undo-balance-update           3 2)
                        (account-undo-nonce-update             3 2)
                        (account-undo-warmth-update            3 2)
                        (account-undo-code-update              3 2)
                        (account-undo-deployment-status-update 3 2)
                        (DOM-SUB-stamps---revert-with-current  3 1)))
