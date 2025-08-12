(module rlptxn)

;; binarity is taken care of via :binary@prove

;; transaction-constancy is taken care of in eponymous section

(defconstraint replay-protection-means-no-chain-id ()
    (if-not-zero TXN
        (if-not-zero txn/CHAIN_ID 
            (eq!  REPLAY_PROTECTION 1)
            (eq!  REPLAY_PROTECTION 0))))

(defconstraint only-frontier-tx-are-not-replay-protection ()
    (if-not-zero TXN
        (if-zero TYPE_0 (eq! REPLAY_PROTECTION 1))))
