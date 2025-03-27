(module ecdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.3 Constraints             ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.3.1 Binary constraints    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; done with binary@prove in columns.lisp

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.3.2 Macro instruction     ;;
;;    decoding and shorthands     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (is_ecrecover)
  (force-bin (+ IS_ECRECOVER_DATA IS_ECRECOVER_RESULT)))

(defun (is_ecadd)
  (force-bin (+ IS_ECADD_DATA IS_ECADD_RESULT)))

(defun (is_ecmul)
  (force-bin (+ IS_ECMUL_DATA IS_ECMUL_RESULT)))

(defun (is_ecpairing)
  (force-bin (+ IS_ECPAIRING_DATA IS_ECPAIRING_RESULT)))

(defun (flag_sum)
  (force-bin (+ (is_ecrecover) (is_ecadd) (is_ecmul) (is_ecpairing))))

(defun (address_sum)
  (+ (* ECRECOVER (is_ecrecover))
     (* ECADD (is_ecadd))
     (* ECMUL (is_ecmul))
     (* ECPAIRING (is_ecpairing))))

(defun (phase_sum)
  (+ (* PHASE_ECRECOVER_DATA IS_ECRECOVER_DATA)
     (* PHASE_ECRECOVER_RESULT IS_ECRECOVER_RESULT)
     (* PHASE_ECADD_DATA IS_ECADD_DATA)
     (* PHASE_ECADD_RESULT IS_ECADD_RESULT)
     (* PHASE_ECMUL_DATA IS_ECMUL_DATA)
     (* PHASE_ECMUL_RESULT IS_ECMUL_RESULT)
     (* PHASE_ECPAIRING_DATA IS_ECPAIRING_DATA)
     (* PHASE_ECPAIRING_RESULT IS_ECPAIRING_RESULT)))

(defun (is_data)
  (force-bin (+ IS_ECRECOVER_DATA IS_ECADD_DATA IS_ECMUL_DATA IS_ECPAIRING_DATA)))

(defun (is_result)
  (force-bin (+ IS_ECRECOVER_RESULT IS_ECADD_RESULT IS_ECMUL_RESULT IS_ECPAIRING_RESULT)))

(defun (transition_to_data)
  (force-bin (* (- 1 (is_data)) (next (is_data)))))

(defun (transition_to_result)
  (force-bin (* (- 1 (is_result)) (next (is_result)))))

(defun (transition_bit)
  (force-bin (+ (transition_to_data) (transition_to_result))))

(defconstraint padding ()
  (if-zero STAMP
           (vanishes! (flag_sum))
           (eq! (flag_sum) 1)))

(defconstraint phase ()
  (eq! PHASE (phase_sum)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.3 constancy conditions ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancy ()
  (begin (stamp-constancy STAMP ID)
         (stamp-constancy STAMP SUCCESS_BIT)
         (stamp-constancy STAMP TOTAL_PAIRINGS)
         (stamp-constancy STAMP ICP)
         (stamp-constancy STAMP (address_sum))))

(defconstraint counter-constancy ()
  (begin (counter-constancy CT CT_MAX)
         (counter-constancy CT IS_INFINITY)
         (counter-constancy CT ACC_PAIRINGS)
         (counter-constancy CT TRIVIAL_PAIRING)
         (counter-constancy CT G2MTR)
         (counter-constancy CT NOT_ON_G2)
         (counter-constancy CT NOT_ON_G2_ACC)
         (counter-constancy INDEX PHASE) ;; NOTE: PHASE, NOT_ON_G2_ACC_MAX, INDEX_MAX are said to be index-constant
         (counter-constancy INDEX NOT_ON_G2_ACC_MAX)
         (counter-constancy INDEX INDEX_MAX)))

(defconstraint pair-of-points-constancy ()
  (if-not-zero ACC_PAIRINGS
               (if-zero (will-remain-constant! ACC_PAIRINGS)
                   (will-remain-constant! ACCPC))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.4 Setting INDEX_MAX    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (index_max_sum)
  (+ (* INDEX_MAX_ECRECOVER_DATA IS_ECRECOVER_DATA)
     (* INDEX_MAX_ECADD_DATA IS_ECADD_DATA)
     (* INDEX_MAX_ECMUL_DATA IS_ECMUL_DATA)
     ;;
     (* INDEX_MAX_ECRECOVER_RESULT IS_ECRECOVER_RESULT)
     (* INDEX_MAX_ECADD_RESULT IS_ECADD_RESULT)
     (* INDEX_MAX_ECMUL_RESULT IS_ECMUL_RESULT)
     (* INDEX_MAX_ECPAIRING_RESULT IS_ECPAIRING_RESULT)))

(defconstraint set-index-max ()
  (eq! (* 16 INDEX_MAX)
       (+ (* 16 (index_max_sum))
          (* IS_ECPAIRING_DATA (- TOTAL_SIZE 16)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.5 Setting TOTAL_SIZE   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-total-size ()
  (eq! TOTAL_SIZE
       (+ (* IS_ECRECOVER_DATA TOTAL_SIZE_ECRECOVER_DATA)
          (* IS_ECADD_DATA TOTAL_SIZE_ECADD_DATA)
          (* IS_ECMUL_DATA TOTAL_SIZE_ECMUL_DATA)
          (* IS_ECPAIRING_DATA TOTAL_SIZE_ECPAIRING_DATA_MIN TOTAL_PAIRINGS)
          (* IS_ECRECOVER_RESULT TOTAL_SIZE_ECRECOVER_RESULT SUCCESS_BIT)
          (* IS_ECADD_RESULT TOTAL_SIZE_ECADD_RESULT SUCCESS_BIT)
          (* IS_ECMUL_RESULT TOTAL_SIZE_ECMUL_RESULT SUCCESS_BIT)
          (* IS_ECPAIRING_RESULT TOTAL_SIZE_ECPAIRING_RESULT SUCCESS_BIT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;  3.3.6 Setting CT, CT_MAX,         ;;
;;  IS_SMALL_POINT and IS_LARGE_POINT ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint small-large-points-flags-sum ()
  (eq! IS_ECPAIRING_DATA (+ IS_SMALL_POINT IS_LARGE_POINT)))

(defconstraint set-ct-max ()
  (eq! CT_MAX
       (+ (* CT_MAX_SMALL_POINT IS_SMALL_POINT) (* CT_MAX_LARGE_POINT IS_LARGE_POINT))))

(defun (transition_from_small_to_large)
  (* IS_SMALL_POINT (next IS_LARGE_POINT)))

(defun (transition_from_large_to_small)
  (* IS_LARGE_POINT (next IS_SMALL_POINT)))

(defun (start-with-small point)
  (if-zero IS_ECPAIRING_DATA
           (eq! (next IS_SMALL_POINT) (next IS_ECPAIRING_DATA))))

(defconstraint set-transitions ()
  (if-not-zero (* IS_ECPAIRING_DATA (next IS_ECPAIRING_DATA))
               (begin (debug (if-not-zero (- CT CT_MAX)
                                          (vanishes! (+ (transition_from_small_to_large)
                                                        (transition_from_large_to_small)))))
                      (if-zero (- CT CT_MAX)
                               (eq! (+ (transition_from_small_to_large) (transition_from_large_to_small))
                                    1)))))

(defconstraint set-ct-outside-ecpairing-data-and-first-row ()
  (if-zero IS_ECPAIRING_DATA
           (begin (vanishes! CT)
                  (vanishes! (next CT)))))

(defconstraint ct-increment-and-reset ()
  (if-eq-else CT CT_MAX
              (eq! (next CT) 0)
              (will-inc! CT 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.7 Setting ACC_PAIRINGS ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-acc-pairings-init ()
  (if-zero IS_ECPAIRING_DATA
           (begin (vanishes! ACC_PAIRINGS)
                  (eq! (next ACC_PAIRINGS) (next IS_ECPAIRING_DATA)))))

(defconstraint set-acc-pairings-increment ()
  (if-not-zero IS_ECPAIRING_DATA
               (eq! (next ACC_PAIRINGS)
                    (* (next IS_ECPAIRING_DATA) (+ ACC_PAIRINGS (transition_from_large_to_small))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.8 Setting ICP          ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-icp-padding ()
  (if-zero (flag_sum)
           (vanishes! ICP)))

(defconstraint set-icp ()
  (let ((internal_checks_passed HURDLE))
       (if-not-zero (transition_to_result)
                    (eq! ICP internal_checks_passed))))

(defconstraint icp-necessary-condition-success-bit ()
  (debug (if-not-zero SUCCESS_BIT
                      (eq! ICP 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.9 Generalities for     ;;
;;        G2MTR                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-g2mtr ()
  (if-not-zero G2MTR
               (begin (eq! ICP 1)
                      (eq! IS_LARGE_POINT 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.10 Generalities for     ;;
;;        ACCPC                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-accpc ()
  (if-not-zero ACCPC
               (begin (eq! SUCCESS_BIT 1)
                      (eq! IS_ECPAIRING_DATA 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;  3.3.11 Setting TRIVIAL_PAIRING  ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-trivial-pairing-outside-ecpairing-data ()
  (if-zero (is_ecpairing)
           (vanishes! TRIVIAL_PAIRING)))

(defconstraint set-trivial-pairing-init ()
  (if-zero (prev IS_ECPAIRING_DATA)
           (if-not-zero IS_ECPAIRING_DATA
                        (eq! TRIVIAL_PAIRING 1))))

(defconstraint conditional-index-constancy ()
  (if-not-zero IS_ECPAIRING_RESULT
           (counter-constancy INDEX TRIVIAL_PAIRING)))

(defconstraint transition-large-to-small ()
  (if-not-zero (transition_from_large_to_small)
               (will-remain-constant! TRIVIAL_PAIRING)))

(defconstraint transition-small-to-large ()
  (if-not-zero (transition_from_small_to_large)
               (if-zero TRIVIAL_PAIRING
                        (vanishes! (next TRIVIAL_PAIRING))
                        (eq! (next TRIVIAL_PAIRING) (next IS_INFINITY)))))

(defconstraint transition-to-result ()
  (if-not-zero (transition_to_result)
               (will-remain-constant! TRIVIAL_PAIRING)))

(defconstraint set-pairing-result-when-trivial-pairngs ()
  (let ((pairing_result_hi (next LIMB))
        (pairing_result_lo (shift LIMB 2)))
       (if-not-zero (transition_to_result)
                    (if-not-zero ICP
                                 (if-zero NOT_ON_G2_ACC_MAX
                                          (if-not-zero TRIVIAL_PAIRING
                                                       (begin (eq! SUCCESS_BIT 1)
                                                              (vanishes! pairing_result_hi)
                                                              (eq! pairing_result_lo 1))))))))

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    3.3.12 Hearbeat ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint vanishing-values ()
  (if-zero (flag_sum)
           (begin (vanishes! INDEX)
                  (vanishes! ID))))

(defconstraint stamp-increment-sanity-check ()
  (begin
    (debug (or! (will-remain-constant! STAMP) (will-inc! STAMP 1))))) ;; implied by the constraint below

(defconstraint stamp-increment ()
  (eq! (next STAMP) (+ STAMP (transition_to_data))))

(defconstraint allowed-transitions ()
  (if-not-zero STAMP
               (eq! (+ (* IS_ECRECOVER_DATA (next (is_ecrecover)))
                       (* IS_ECADD_DATA (next (is_ecadd)))
                       (* IS_ECMUL_DATA (next (is_ecmul)))
                       (* IS_ECPAIRING_DATA (next (is_ecpairing)))
                       (* IS_ECRECOVER_RESULT (next IS_ECRECOVER_RESULT))
                       (* IS_ECADD_RESULT (next IS_ECADD_RESULT))
                       (* IS_ECMUL_RESULT (next IS_ECMUL_RESULT))
                       (* IS_ECPAIRING_RESULT (next IS_ECPAIRING_RESULT))
                       (transition_to_data))
                    1)))

(defconstraint index-reset ()
  (if-not-zero (transition_bit)
               (vanishes! (next INDEX))))

(defconstraint index-increment ()
  (if-not-zero STAMP
               (if-eq-else INDEX INDEX_MAX
                           (eq! (transition_bit) 1)
                           (eq! (next INDEX) (+ 1 INDEX)))))

(defconstraint final-row (:domain {-1})
  (if-not-zero STAMP
               (begin (eq! (is_result) 1)
                      (eq! INDEX INDEX_MAX))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.13 ID increment        ;;
;;         constraints         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint id-increment ()
  (if-not-zero (- (next STAMP) STAMP)
               (eq! (next ID)
                    (+ ID
                       1
                       (+ (* 256 256 256 (next BYTE_DELTA))
                          (* 256 256 (shift BYTE_DELTA 2))
                          (* 256 (shift BYTE_DELTA 3))
                          (shift BYTE_DELTA 4))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  3.3.14 Setting NOT_ON_G2   ;;
;;         and NOT_ON_G2_ACC   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint large-point-necessary-condition-not-on-g2 ()
  (if-not-zero NOT_ON_G2
               (eq! IS_LARGE_POINT 1)))

(defconstraint set-not-on-g2-not-on-g2-acc ()
  (if-zero IS_ECPAIRING_DATA
           (begin (vanishes! NOT_ON_G2_ACC)
                  (vanishes! (next NOT_ON_G2_ACC)))))

(defconstraint set-not-on-g2-acc-during-transition-small-to-large ()
  (if-not-zero (transition_from_small_to_large)
               (eq! (next NOT_ON_G2_ACC)
                    (+ NOT_ON_G2_ACC (next NOT_ON_G2)))))

(defconstraint set-not-on-g2-acc-during-transition-large-to-small ()
  (if-not-zero (transition_from_large_to_small)
               (will-remain-constant! NOT_ON_G2_ACC)))

(defconstraint ecpairing-data-icp-necessary-conditions-not-on-g2-acc-max ()
  (if-not-zero NOT_ON_G2_ACC_MAX
               (begin (eq! IS_ECPAIRING_DATA 1)
                      (eq! ICP 1))))

(defconstraint set-not-on-g2-acc-max ()
  (if-not-zero IS_ECPAIRING_DATA
               (if-not-zero (next IS_ECPAIRING_RESULT)
                            (eq! NOT_ON_G2_ACC_MAX NOT_ON_G2_ACC))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.4 Utilities       ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.4.1 WCP utilities ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (callToLT k a b c d)
  (begin (eq! (shift WCP_FLAG k) 1)
         (eq! (shift WCP_INST k) EVM_INST_LT)
         (eq! (shift WCP_ARG1_HI k) a)
         (eq! (shift WCP_ARG1_LO k) b)
         (eq! (shift WCP_ARG2_HI k) c)
         (eq! (shift WCP_ARG2_LO k) d)))

(defun (callToEQ k a b c d)
  (begin (eq! (shift WCP_FLAG k) 1)
         (eq! (shift WCP_INST k) EVM_INST_EQ)
         (eq! (shift WCP_ARG1_HI k) a)
         (eq! (shift WCP_ARG1_LO k) b)
         (eq! (shift WCP_ARG2_HI k) c)
         (eq! (shift WCP_ARG2_LO k) d)))

(defun (callToISZERO k a b)
  (begin (eq! (shift WCP_FLAG k) 1)
         (eq! (shift WCP_INST k) EVM_INST_ISZERO)
         (eq! (shift WCP_ARG1_HI k) a)
         (eq! (shift WCP_ARG1_LO k) b)
         (debug (vanishes! (shift WCP_ARG2_HI k)))
         (debug (vanishes! (shift WCP_ARG2_LO k)))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.4.2 EXT utilities ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (callToADDMOD k a b c d e f)
  (begin (eq! (shift EXT_FLAG k) 1)
         (eq! (shift EXT_INST k) ADDMOD)
         (eq! (shift EXT_ARG1_HI k) a)
         (eq! (shift EXT_ARG1_LO k) b)
         (eq! (shift EXT_ARG2_HI k) c)
         (eq! (shift EXT_ARG2_LO k) d)
         (eq! (shift EXT_ARG3_HI k) e)
         (eq! (shift EXT_ARG3_LO k) f)))

(defun (callToMULMOD k a b c d e f)
  (begin (eq! (shift EXT_FLAG k) 1)
         (eq! (shift EXT_INST k) MULMOD)
         (eq! (shift EXT_ARG1_HI k) a)
         (eq! (shift EXT_ARG1_LO k) b)
         (eq! (shift EXT_ARG2_HI k) c)
         (eq! (shift EXT_ARG2_LO k) d)
         (eq! (shift EXT_ARG3_HI k) e)
         (eq! (shift EXT_ARG3_LO k) f)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.4.3 C1 membership ;;
;;       utilities     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (callToC1Membership k P_x_hi P_x_lo P_y_hi P_y_lo)
  (let ((P_x_is_in_range (shift WCP_RES k))
        (P_y_is_in_range (shift WCP_RES (+ k 1)))
        (P_satisfies_cubic (shift WCP_RES (+ k 2)))
        (P_y_square_hi (shift EXT_RES_HI k))
        (P_y_square_lo (shift EXT_RES_LO k))
        (P_x_square_hi (shift EXT_RES_HI (+ k 1)))
        (P_x_square_lo (shift EXT_RES_LO (+ k 1)))
        (P_x_cube_hi (shift EXT_RES_HI (+ k 2)))
        (P_x_cube_lo (shift EXT_RES_LO (+ k 2)))
        (P_x_cube_plus_three_hi (shift EXT_RES_HI (+ k 3)))
        (P_x_cube_plus_three_lo (shift EXT_RES_LO (+ k 3)))
        (P_is_in_range (shift HURDLE (+ k 1)))
        (C1_membership (shift HURDLE k))
        (large_sum (+ P_x_hi P_x_lo P_y_hi P_y_lo))
        (P_is_point_at_infinity (shift IS_INFINITY k)))
       (begin (callToC1MembershipWCP k
                                     P_x_hi
                                     P_x_lo
                                     P_y_hi
                                     P_y_lo
                                     P_y_square_hi
                                     P_y_square_lo
                                     P_x_cube_plus_three_hi
                                     P_x_cube_plus_three_lo)
              (callToC1MembershipEXT k
                                     P_x_hi
                                     P_x_lo
                                     P_y_hi
                                     P_y_lo
                                     P_x_square_hi
                                     P_x_square_lo
                                     P_x_cube_hi
                                     P_x_cube_lo)
              (eq! P_is_in_range (* P_x_is_in_range P_y_is_in_range))
              (eq! C1_membership
                   (* P_is_in_range (+ P_is_point_at_infinity P_satisfies_cubic)))
              (if-zero P_is_in_range
                       (vanishes! P_is_point_at_infinity)
                       (if-zero large_sum
                                (eq! P_is_point_at_infinity 1)
                                (vanishes! P_is_point_at_infinity))))))

;; Note: in the specs for simplicity we omit the last four arguments
(defun (callToC1MembershipWCP k
                              _P_x_hi
                              _P_x_lo
                              _P_y_hi
                              _P_y_lo
                              _P_y_square_hi
                              _P_y_square_lo
                              _P_x_cube_plus_three_hi
                              _P_x_cube_plus_three_lo)
  (begin (callToLT k _P_x_hi _P_x_lo P_BN_HI P_BN_LO)
         (callToLT (+ k 1) _P_y_hi _P_y_lo P_BN_HI P_BN_LO)
         (callToEQ (+ k 2) _P_y_square_hi _P_y_square_lo _P_x_cube_plus_three_hi _P_x_cube_plus_three_lo)))

;; Note: in the specs for simplicity we omit the last four arguments
(defun (callToC1MembershipEXT k
                              _P_x_hi
                              _P_x_lo
                              _P_y_hi
                              _P_y_lo
                              _P_x_square_hi
                              _P_x_square_lo
                              _P_x_cube_hi
                              _P_x_cube_lo)
  (begin (callToMULMOD k _P_y_hi _P_y_lo _P_y_hi _P_y_lo P_BN_HI P_BN_LO)
         (callToMULMOD (+ k 1) _P_x_hi _P_x_lo _P_x_hi _P_x_lo P_BN_HI P_BN_LO)
         (callToMULMOD (+ k 2) _P_x_square_hi _P_x_square_lo _P_x_hi _P_x_lo P_BN_HI P_BN_LO)
         (callToADDMOD (+ k 3) _P_x_cube_hi _P_x_cube_lo 0 3 P_BN_HI P_BN_LO)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.4.4 Well formed   ;;
;;       coordinates   ;;
;;       utilities     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (callToWellFormedCoordinates k
                                    B_x_Im_hi
                                    B_x_Im_lo
                                    B_x_Re_hi
                                    B_x_Re_lo
                                    B_y_Im_hi
                                    B_y_Im_lo
                                    B_y_Re_hi
                                    B_y_Re_lo)
  (let ((B_x_Im_is_in_range (shift WCP_RES k))
        (B_x_Re_is_in_range (shift WCP_RES (+ k 1)))
        (B_y_Im_is_in_range (shift WCP_RES (+ k 2)))
        (B_y_Re_is_in_range (shift WCP_RES (+ k 3)))
        (B_x_is_in_range (shift HURDLE (+ k 2)))
        (B_y_is_in_range (shift HURDLE (+ k 1)))
        (well_formed_coordinates (shift HURDLE k))
        (B_is_point_at_infinity (shift IS_INFINITY k))
        (very_large_sum (+ B_x_Im_hi B_x_Im_lo B_x_Re_hi B_x_Re_lo B_y_Im_hi B_y_Im_lo B_y_Re_hi B_y_Re_lo)))
       (begin (callToLT k B_x_Im_hi B_x_Im_lo P_BN_HI P_BN_LO)
              (callToLT (+ k 1) B_x_Re_hi B_x_Re_lo P_BN_HI P_BN_LO)
              (callToLT (+ k 2) B_y_Im_hi B_y_Im_lo P_BN_HI P_BN_LO)
              (callToLT (+ k 3) B_y_Re_hi B_y_Re_lo P_BN_HI P_BN_LO)
              (eq! B_x_is_in_range (* B_x_Im_is_in_range B_x_Re_is_in_range))
              (eq! B_y_is_in_range (* B_y_Im_is_in_range B_y_Re_is_in_range))
              (eq! well_formed_coordinates (* B_x_is_in_range B_y_is_in_range))
              (if-zero well_formed_coordinates
                       (vanishes! B_is_point_at_infinity)
                       (if-zero very_large_sum
                                (eq! B_is_point_at_infinity 1)
                                (vanishes! B_is_point_at_infinity))))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.6 Specialized     ;;
;;     constraints     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.6.1 The ECRECOVER ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecrecover-hypothesis)
  (* IS_ECRECOVER_DATA
     (~ (- ID (prev ID)))))

(defconstraint internal-checks-ecrecover (:guard (ecrecover-hypothesis))
  (let ((h_hi LIMB)
        (h_lo (next LIMB))
        (v_hi (shift LIMB 2))
        (v_lo (shift LIMB 3))
        (r_hi (shift LIMB 4))
        (r_lo (shift LIMB 5))
        (s_hi (shift LIMB 6))
        (s_lo (shift LIMB 7)))
       (begin (callToLT 0 r_hi r_lo SECP256K1N_HI SECP256K1N_LO)
              (callToLT 1 0 0 r_hi r_lo)
              (callToLT 2 s_hi s_lo SECP256K1N_HI SECP256K1N_LO)
              (callToLT 3 0 0 s_hi s_lo)
              (callToEQ 4 v_hi v_lo 0 27)
              (callToEQ 5 v_hi v_lo 0 28))))

(defconstraint justify-success-bit-ecrecover (:guard (ecrecover-hypothesis))
  (let ((r_is_in_range WCP_RES)
        (r_is_positive (next WCP_RES))
        (s_is_in_range (shift WCP_RES 2))
        (s_is_positive (shift WCP_RES 3))
        (v_is_27 (shift WCP_RES 4))
        (v_is_28 (shift WCP_RES 5))
        (internal_checks_passed (shift HURDLE INDEX_MAX_ECRECOVER_DATA)))
       (begin (eq! HURDLE (* r_is_in_range r_is_positive))
              (eq! (next HURDLE) (* s_is_in_range s_is_positive))
              (eq! (shift HURDLE 2)
                   (* HURDLE (next HURDLE)))
              (eq! internal_checks_passed
                   (* (shift HURDLE 2) (+ v_is_27 v_is_28)))
              (if-zero internal_checks_passed
                       (vanishes! SUCCESS_BIT)))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.6.2 The ECADD     ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecadd-hypothesis)
  (* IS_ECADD_DATA
     (~ (- ID (prev ID)))))

(defconstraint internal-checks-ecadd (:guard (ecadd-hypothesis))
  (let ((P_x_hi LIMB)
        (P_x_lo (next LIMB))
        (P_y_hi (shift LIMB 2))
        (P_y_lo (shift LIMB 3))
        (Q_x_hi (shift LIMB 4))
        (Q_x_lo (shift LIMB 5))
        (Q_y_hi (shift LIMB 6))
        (Q_y_lo (shift LIMB 7)))
       (begin (callToC1Membership 0 P_x_hi P_x_lo P_y_hi P_y_lo)
              (callToC1Membership 4 Q_x_hi Q_x_lo Q_y_hi Q_y_lo))))

(defconstraint justify-success-bit-ecadd (:guard (ecadd-hypothesis))
  (let ((C1_membership_first_point HURDLE)
        (C1_membership_second_point (shift HURDLE 4))
        (internal_checks_passed (shift HURDLE INDEX_MAX_ECADD_DATA)))
       (begin (eq! internal_checks_passed (* C1_membership_first_point C1_membership_second_point))
              (eq! SUCCESS_BIT internal_checks_passed))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.6.3 The ECMUL     ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecmul-hypothesis)
  (* IS_ECMUL_DATA
     (~ (- ID (prev ID)))))

(defconstraint internal-checks-ecmul (:guard (ecmul-hypothesis))
  (let ((P_x_hi LIMB)
        (P_x_lo (next LIMB))
        (P_y_hi (shift LIMB 2))
        (P_y_lo (shift LIMB 3))
        (n_hi (shift LIMB 4))
        (n_lo (shift LIMB 5)))
       (begin (callToC1Membership 0 P_x_hi P_x_lo P_y_hi P_y_lo))))

(defconstraint justify-success-bit-ecmul (:guard (ecmul-hypothesis))
  (let ((C1_membership HURDLE)
        (internal_checks_passed (shift HURDLE INDEX_MAX_ECMUL_DATA)))
       (begin (eq! internal_checks_passed C1_membership)
              (eq! SUCCESS_BIT internal_checks_passed))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.6.4 The ECPAIRING ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecpairing-hypothesis)
  (* IS_ECPAIRING_DATA
     (- ACC_PAIRINGS (prev ACC_PAIRINGS))))

(defconstraint internal-checks-ecpairing (:guard (ecpairing-hypothesis))
  (let ((A_x_hi LIMB)
        (A_x_lo (next LIMB))
        (A_y_hi (shift LIMB 2))
        (A_y_lo (shift LIMB 3))
        (B_x_Im_hi (shift LIMB 4))
        (B_x_Im_lo (shift LIMB 5))
        (B_x_Re_hi (shift LIMB 6))
        (B_x_Re_lo (shift LIMB 7))
        (B_y_Im_hi (shift LIMB 8))
        (B_y_Im_lo (shift LIMB 9))
        (B_y_Re_hi (shift LIMB 10))
        (B_y_Re_lo (shift LIMB 11)))
       (begin (callToC1Membership 0 A_x_hi A_x_lo A_y_hi A_y_lo)
              (callToWellFormedCoordinates 4
                                           B_x_Im_hi
                                           B_x_Im_lo
                                           B_x_Re_hi
                                           B_x_Re_lo
                                           B_y_Im_hi
                                           B_y_Im_lo
                                           B_y_Re_hi
                                           B_y_Re_lo))))

(defconstraint propagation-of-internal-checks-passed (:guard (ecpairing-hypothesis))
  (let ((C1_membership HURDLE)
        (well_formed_coordinates (shift HURDLE 4))
        (internal_checks_passed (shift HURDLE INDEX_MAX_ECPAIRING_DATA_MIN))
        (prev_internal_checks_passed (shift HURDLE -1)))
       (begin (if-zero (- ACC_PAIRINGS 1)
                       (eq! internal_checks_passed (* C1_membership well_formed_coordinates))
                       (begin (eq! (shift HURDLE 10) (* C1_membership well_formed_coordinates))
                              (eq! internal_checks_passed
                                   (* (shift HURDLE 10) prev_internal_checks_passed)))))))

(defconstraint justify-success-bit-ecpairing (:guard (ecpairing-hypothesis))
  (if-zero ICP
           (vanishes! SUCCESS_BIT)
           (eq! SUCCESS_BIT (- 1 NOT_ON_G2_ACC_MAX))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.7 Elliptic curve  ;;
;;      circuit flags  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;  3.7.1 G2 non membership  ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint g2-membership-flags ()
  (if-not-zero NOT_ON_G2_ACC_MAX
               (begin (vanishes! ACCPC)
                      (eq! G2MTR NOT_ON_G2)
                      (if-not-zero G2MTR
                                   (vanishes! IS_INFINITY)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;  3.7.2 Succeseful ECPAIRING  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint successful-ecpairing-flags (:guard (ecpairing-hypothesis))
  (let ((small_point_is_at_infinity IS_INFINITY)
        (large_point_is_at_infinity (shift IS_INFINITY 4)))
       (if-not-zero SUCCESS_BIT
                    (if-not-zero large_point_is_at_infinity
                                 (begin (vanishes! (shift G2MTR 4))
                                        (vanishes! ACCPC))
                                 (begin (eq! (shift G2MTR 4) small_point_is_at_infinity)
                                        (eq! ACCPC (- 1 small_point_is_at_infinity)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 3.7.3 Interface for ;;
;;       Gnark         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint ecrecover-circuit-selector ()
  (eq! CS_ECRECOVER (* ICP (is_ecrecover))))

(defconstraint ecadd-circuit-selector ()
  (eq! CS_ECADD (* ICP (is_ecadd))))

(defconstraint ecmul-circuit-selector ()
  (eq! CS_ECMUL (* ICP (is_ecmul))))

(defconstraint ecpairing-circuit-selector ()
  (begin 
    (if-not-zero IS_ECPAIRING_DATA (eq! CS_ECPAIRING ACCPC))
    (if-not-zero IS_ECPAIRING_RESULT (eq! CS_ECPAIRING (* SUCCESS_BIT (- 1 TRIVIAL_PAIRING))))
    (if-zero (is_ecpairing) (vanishes! CS_ECPAIRING))
  )
)

(defconstraint g2-membership-circuit-selector ()
  (eq! CS_G2_MEMBERSHIP G2MTR))

(defconstraint circuit-selectors-sum-binary ()
  (debug (is-binary (+ CS_ECRECOVER CS_ECADD CS_ECMUL CS_ECPAIRING CS_G2_MEMBERSHIP))))


