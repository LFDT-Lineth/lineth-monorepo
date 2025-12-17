(module blsdata)

(defun (first_row_of_new_first_input)
    (* (- 1 (prev IS_FIRST_INPUT)) IS_FIRST_INPUT))

(defun (first_row_of_new_second_input)
    (* (- 1 (prev IS_SECOND_INPUT)) IS_SECOND_INPUT))

(defun (first_row_of_new_input)
    (+ (first_row_of_new_first_input) (first_row_of_new_second_input)))