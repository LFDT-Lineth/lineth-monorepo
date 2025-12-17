(module blsdata)

(defconstraint set-mint-bit-and-mint-acc-when-is-data-is-zero ()
    (if-zero (is_data)
        (begin 
            (vanishes! MINT_BIT)
            (vanishes! MINT_ACC)
        )))

(defconstraint propagate-mint-bit-into-mint-acc ()
    (if-not-zero (+ (transition_to_data) (will_switch_from_first_to_second) (will_switch_from_second_to_first))
        (begin (if-zero MINT_ACC
                    (eq! (next MINT_ACC) (next MINT_BIT)))
               (if-not-zero MINT_ACC
                    (eq! (next MINT_ACC) 1)))))

(defconstraint propagate-mint-acc-into-mint ()
    (if-not-zero (transition_to_result)
        (eq! MINT MINT_ACC)))