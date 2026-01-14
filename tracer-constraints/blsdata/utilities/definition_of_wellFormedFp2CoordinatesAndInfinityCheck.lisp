(module blsdata)

(defun (wellFormedFp2CoordinatesAndInfinityCheck k P_x_Im_3 P_x_Im_2 P_x_Im_1 P_x_Im_0 P_x_Re_3 P_x_Re_2 P_x_Re_1 P_x_Re_0 P_y_Im_3 P_y_Im_2 P_y_Im_1 P_y_Im_0 P_y_Re_3 P_y_Re_2 P_y_Re_1 P_y_Re_0)
    (let ((P_x_Im_is_in_range (shift WCP_RES k))
          (P_x_Re_is_in_range (shift WCP_RES (+ k 4)))
          (P_y_Im_is_in_range (shift WCP_RES (+ k 8)))
          (P_y_Re_is_in_range (shift WCP_RES (+ k 12)))
          (well_formed_coordinates (- 1 MINT_BIT)))
    (let ((P_x_is_in_range (* P_x_Im_is_in_range P_x_Re_is_in_range))
          (P_y_is_in_range (* P_y_Im_is_in_range P_y_Re_is_in_range)))
    (begin
        (wcpGeneralizedCallToLT k P_x_Im_3 P_x_Im_2 P_x_Im_1 P_x_Im_0 BLS_PRIME_3 BLS_PRIME_2 BLS_PRIME_1 BLS_PRIME_0)
        (wcpGeneralizedCallToLT (+ k 4) P_x_Re_3 P_x_Re_2 P_x_Re_1 P_x_Re_0 BLS_PRIME_3 BLS_PRIME_2 BLS_PRIME_1 BLS_PRIME_0)
        (wcpGeneralizedCallToLT (+ k 8) P_y_Im_3 P_y_Im_2 P_y_Im_1 P_y_Im_0 BLS_PRIME_3 BLS_PRIME_2 BLS_PRIME_1 BLS_PRIME_0)
        (wcpGeneralizedCallToLT (+ k 12) P_y_Re_3 P_y_Re_2 P_y_Re_1 P_y_Re_0 BLS_PRIME_3 BLS_PRIME_2 BLS_PRIME_1 BLS_PRIME_0)
        (eq! well_formed_coordinates (* P_x_is_in_range P_y_is_in_range))
        (isInfinity k (+ P_x_Re_3 P_x_Re_2 P_x_Re_1 P_x_Re_0 
                         P_x_Im_3 P_x_Im_2 P_x_Im_1 P_x_Im_0 
                         P_y_Re_3 P_y_Re_2 P_y_Re_1 P_y_Re_0 
                         P_y_Im_3 P_y_Im_2 P_y_Im_1 P_y_Im_0))
                        ))))
