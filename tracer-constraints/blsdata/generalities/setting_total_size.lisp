(module blsdata)

(defconstraint setting-total-size-data ()
    (if-not-zero (is_data)
        (if-zero (is_variable_size_data)
            (eq! TOTAL_SIZE (* (+ INDEX_MAX 1) 16)))))

(defconstraint setting-total-size-result ()
    (if-not-zero (is_result)
        (eq! TOTAL_SIZE (* (+ INDEX_MAX 1) 16 SUCCESS_BIT))))