(module hub)

(defun (deployment)      (next context/BYTE_CODE_DEPLOYMENT_STATUS))
(defun (code-address-hi) (next context/BYTE_CODE_ADDRESS_HI))
(defun (code-address-lo) (next context/BYTE_CODE_ADDRESS_LO))
(defun (will-revert)     CONTEXT_WILL_REVERT)

(defun (stop-instruction) (* PEEK_AT_STACK
                             stack/HALT_FLAG
                             [ stack/DEC_FLAG 3 ]))

(defconstraint stop-stack-pattern (:guard (stop-instruction))
               (stack-pattern-0-0))

(defconstraint stop-gas-cost (:guard (stop-instruction))
               (vanishes! GAS_COST))

(defconstraint stop-raises-no-exceptions (:guard (stop-instruction))
               (vanishes! XAHOY))

(defconstraint stop-first-non-stack-row (:guard (stop-instruction))
               (begin
                 (will-eq!          PEEK_AT_CONTEXT 1)
                 (read-context-data 1 CONTEXT_NUMBER)))

(defconstraint stop-setting-NSR-and-peeking-flags (:guard (stop-instruction))
               (begin
                 (eq! NSR
                      (+ 2
                         (* (deployment)
                            (+ 1 (will-revert)))))
                 (if-zero (force-bin (deployment))
                          (begin
                            (debug (eq! NSR 2) )
                            (eq! NSR
                                 (+ (shift PEEK_AT_CONTEXT 1)
                                    (shift PEEK_AT_CONTEXT 2)))
                            (execution-provides-empty-return-data 2))
                          (if-zero (force-bin (will-revert))
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


(defconstraint stop-first-address-row (:guard (stop-instruction))
               (begin (debug (vanishes!       (shift account/TRM_FLAG     2)))
                      (debug (vanishes!       (shift account/ROMLEX_FLAG  2)))
                      (eq! (shift account/ADDRESS_HI  2) (code-address-hi))
                      (eq! (shift account/ADDRESS_LO  2) (code-address-lo))
                      (account-same-balance    2)
                      (account-same-nonce      2)
                      (account-same-warmth     2)
                      (vanishes! (shift account/CODE_SIZE_NEW    2))
                      (eq!       (shift account/CODE_HASH_HI_NEW 2) EMPTY_KECCAK_HI)
                      (eq!       (shift account/CODE_HASH_LO_NEW 2) EMPTY_KECCAK_LO)
                      (account-same-deployment-number 2)
                      (eq!       (shift account/DEPLOYMENT_STATUS     2) 1)
                      (eq!       (shift account/DEPLOYMENT_STATUS_NEW 2) 0)
                      (standard-dom-sub-stamps 2 0)))

(defconstraint stop-second-address-row (:guard (stop-instruction))
               (begin (debug (vanishes! (shift account/TRM_FLAG     3)))
                      (debug (vanishes! (shift account/ROMLEX_FLAG  3)))
                      (account-same-address-as               3 2)
                      (account-undo-balance-update           3 2)
                      (account-undo-nonce-update             3 2)
                      (account-undo-warmth-update            3 2)
                      (account-undo-code-update              3 2)
                      (account-undo-deployment-status-update 3 2)
                      (revert-dom-sub-stamps                 3 1)))
