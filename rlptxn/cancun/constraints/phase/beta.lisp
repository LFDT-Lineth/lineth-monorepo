(module rlptxn)


(defun    (first-row-of-beta-phase)                           (*   IS_BETA                   TXN))
(defun    (first-row-of-beta-phase-with-replay-protection)    (*   (first-row-of-beta-phase) REPLAY_PROTECTION))
(defun    (beta-phase---Tw-scalar)     (if-zero   REPLAY_PROTECTION
                                                  ;; replay_protection ≡ <false>
                                                  (+ UNPROTECTED_V  Y_PARITY)
                                                  ;; replay_protection ≡ <true>
                                                  (+ (* 2 txn/CHAIN_ID)
                                                     PROTECTED_BASE_V
                                                     Y_PARITY)))

(defconstraint    beta-phase---RLP-ization-of-Tw
                  (:guard   (first-row-of-beta-phase))
                  (let ((ROFF 1))
                    (begin
                      (limb-of-lt-only                     ROFF)
                      (rlp-compound-constraint---INTEGER   ROFF
                                                           0
                                                           (beta-phase---Tw-scalar)
                                                           (- 1 REPLAY_PROTECTION)))))

(defconstraint    beta-phase---RLP-ization-of-the-transactions-chain-id
                  (:guard (first-row-of-beta-phase-with-replay-protection))
                  (let  ((ROFF   (+ 3 1)))
                    (begin
                      (limb-of-lx-only                     ROFF)
                      (rlp-compound-constraint---INTEGER   ROFF
                                                           0
                                                           txn/CHAIN_ID
                                                           0))))

(defconstraint    beta-phase---accounting-for-RLP-empty-RLP-empty-in-the-RLP-string
                  (:guard (first-row-of-beta-phase-with-replay-protection))
                  (let  ((ROFF   (+ 3 3 1)))
                  (begin
                    (limb-of-lx-only  ROFF)
                    (set-limb         ROFF
                                      (+ (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO))
                                         (* RLP_PREFIX_INT_SHORT (^ 256 14))) 
                                      2)
                    (eq! (shift PHASE_END ROFF) 1)
                    )))
