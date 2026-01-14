(module blsdata)

(defun (two_input_type_prc_data)
    (+ DATA_BLS_G1_ADD_FLAG
       DATA_BLS_G1_MSM_FLAG
       DATA_BLS_G2_ADD_FLAG
       DATA_BLS_G2_MSM_FLAG
       DATA_BLS_PAIRING_CHECK_FLAG))

(defun (one_input_type_prc_data)
    (+ DATA_POINT_EVALUATION_FLAG
       DATA_BLS_MAP_FP_TO_G1_FLAG
       DATA_BLS_MAP_FP2_TO_G2_FLAG))

(defun (will_switch_from_first_to_second)
    (* IS_FIRST_INPUT (next IS_SECOND_INPUT)))

(defun (will_switch_from_second_to_first)
    (* IS_SECOND_INPUT (next IS_FIRST_INPUT)))

(defconstraint counter-constancy-first-and-second ()
     (begin (counter-constancy CT IS_FIRST_INPUT)
            (counter-constancy CT IS_SECOND_INPUT))) ;; TODO: add to constancy conditions in both lisp and specs

(defconstraint either-first-or-second ()
    (eq! (is_data) (+ IS_FIRST_INPUT IS_SECOND_INPUT)))

(defconstraint data-start-with-first-input ()
    (if-zero (is_data)
        (eq! (next IS_FIRST_INPUT) (next (is_data)))))

(defconstraint one-input-is-first-input ()
    (if-not-zero (one_input_type_prc_data)
        (eq! IS_FIRST_INPUT 1)))

(defconstraint two-input-is-either-first-or-second ()
    (if-not-zero (two_input_type_prc_data)
        (eq! (+ IS_FIRST_INPUT IS_SECOND_INPUT) 1)))

(defconstraint two-input-transitions ()
    (if-not-zero (two_input_type_prc_data)
        (if-eq-else CT CT_MAX
            (eq! (+ (will_switch_from_first_to_second)
                    (will_switch_from_second_to_first))
                 (next (two_input_type_prc_data)))
            (vanishes! (+ (will_switch_from_first_to_second) 
                          (will_switch_from_second_to_first))))))




