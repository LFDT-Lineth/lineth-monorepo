(module blsdata)

(defun (is_variable_size_data)
    (+ DATA_BLS_G1_MSM_FLAG
       DATA_BLS_G2_MSM_FLAG
       DATA_BLS_PAIRING_CHECK_FLAG))

(defconstraint acc-inputs-init ()
    (if-zero (is_variable_size_data)
        (begin (vanishes! ACC_INPUTS)
               (eq! (next ACC_INPUTS) (next (is_variable_size_data))))))

(defconstraint acc-inputs-increment ()
    (if-not-zero (is_variable_size_data)
        (if-eq-else (next (is_variable_size_data)) 0
            (vanishes! (next ACC_INPUTS))
            (eq! (next ACC_INPUTS)
                 (+ ACC_INPUTS
                    (will_switch_from_second_to_first))))))