(module hub)

(defun (illegal-precompiles)
 (force-bin (* PEEK_AT_SCENARIO (+ scenario/PRC_RIPEMD-160 scenario/PRC_BLAKE2f))))

(defcomputedcolumn (PROVER_ILLEGAL_TRANSACTION_DETECTED_ACC :i16 :fwd)
    (* USER (+ (prev PROVER_ILLEGAL_TRANSACTION_DETECTED_ACC)
               (illegal-precompiles))))

(defcomputedcolumn (PROVER_ILLEGAL_TRANSACTION_DETECTED :binary :bwd)
     (if-not-zero (system-txn-numbers---user-txn-end)
          ;; finalization constraint
          (~ PROVER_ILLEGAL_TRANSACTION_DETECTED_ACC)
          ;; bwd propagation
          (*  USER (next PROVER_ILLEGAL_TRANSACTION_DETECTED))))
