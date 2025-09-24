(module blsdata)

(defun (isInfinity k coordinate_sum)
    (let ((P_is_point_at_infinity (shift IS_INFINITY k)))
        (begin 
            (if-zero coordinate_sum
                (eq! P_is_point_at_infinity 1))
            (if-not-zero coordinate_sum
                (eq! P_is_point_at_infinity 0)))))