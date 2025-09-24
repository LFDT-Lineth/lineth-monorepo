(module blsdata)

(defun (wellFormedFpCoordinatesAndInfinityCheck k P_x_3 P_x_2 P_x_1 P_x_0 P_y_3 P_y_2 P_y_1 P_y_0)
    (let ((P_x_is_in_range (shift WCP_RES k))
          (P_y_is_in_range (shift WCP_RES (+ k 4)))
          (well_formed_coordinates (- 1 MINT_BIT)))
    (begin 
        (wcpGeneralizedCallToLT k P_x_3 P_x_2 P_x_1 P_x_0 BLS_PRIME_3 BLS_PRIME_2 BLS_PRIME_1 BLS_PRIME_0) 
        (wcpGeneralizedCallToLT (+ k 4) P_y_3 P_y_2 P_y_1 P_y_0 BLS_PRIME_3 BLS_PRIME_2 BLS_PRIME_1 BLS_PRIME_0) 
        (eq! well_formed_coordinates (* P_x_is_in_range P_y_is_in_range))
        (isInfinity k (+ P_x_3 P_x_2 P_x_1 P_x_0 P_y_3 P_y_2 P_y_1 P_y_0))
    )))