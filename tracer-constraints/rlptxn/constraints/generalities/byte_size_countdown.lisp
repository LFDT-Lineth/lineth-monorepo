(module rlptxn)

(defconstraint byte-size-countdown-update-constraints ()
    (if-zero IS_RLP_PREFIX
        (begin
        (eq! LT_BYTE_SIZE_COUNTDOWN 
             (- (prev LT_BYTE_SIZE_COUNTDOWN)
                (* LC LT cmp/LIMB_SIZE)))
        (eq! LX_BYTE_SIZE_COUNTDOWN 
             (- (prev LX_BYTE_SIZE_COUNTDOWN)
                (* LC LX cmp/LIMB_SIZE))))))

(defconstraint byte-size-countdown-finalization-constraints ()
    (if-not-zero (* IS_S PHASE_END)
        (begin
        (vanishes! LT_BYTE_SIZE_COUNTDOWN)
        (vanishes! LX_BYTE_SIZE_COUNTDOWN))))

(defcomputedcolumn (TO_HASH_BY_PROVER :binary) (* LC LX))
