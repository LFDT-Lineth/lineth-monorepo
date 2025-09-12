(module blsdata)

(defconstraint if-mint-is-one-then-mext-is-not-relevant ()
    (if-not-zero MINT
        (begin
            (vanishes! MEXT_BIT)
            (debug (vanishes! MEXT_ACC))
            (debug (vanishes! MEXT)))))

(defconstraint if-is-data-is-zero-then-mext-bit-and-mext-acc-vanish ()
    (if-zero (is_data)
        (begin 
            (vanishes! MEXT_BIT)
            (vanishes! MEXT_ACC))))

(defconstraint if-is-map-fp-to-g1-is-one-then-mext-bit-and-mext-acc-and-mext-vanish ()
    (if-not-zero (is_map_fp_to_g1)
        (begin
            (vanishes! MEXT_BIT)
            (debug (vanishes! MEXT_ACC))
            (debug (vanishes! MEXT)))))

(defconstraint if-is-map-fp2-to-g2-is-one-then-mext-bit-and-mext-acc-and-mext-vanish ()
    (if-not-zero (is_map_fp2_to_g2)
        (begin
            (vanishes! MEXT_BIT)
            (debug (vanishes! MEXT_ACC))
            (debug (vanishes! MEXT)))))

(defconstraint propagate-mext-bit-into-mext-acc ()
    (if-not-zero (+ (transition_to_data) (will_switch_from_first_to_second) (will_switch_from_second_to_first))
        (eq! (next MEXT_ACC) (+ MEXT_ACC (next MEXT_BIT)))))

(defconstraint propagate-mext-acc-into-mext ()
    (if-not-zero (transition_to_result)
        (eq! MEXT MEXT_ACC)))