(module ecdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    1.3 Constraints             ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    1.3.1 Binary constraints    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint binary-constraints ()
  (begin (is-binary IS_ECRECOVER_DATA)
         (is-binary IS_ECRECOVER_RESULT)
         (is-binary IS_ECADD_DATA)
         (is-binary IS_ECADD_RESULT)
         (is-binary IS_ECMUL_DATA)
         (is-binary IS_ECMUL_RESULT)
         (is-binary IS_ECPAIRING_DATA)
         (is-binary IS_ECPAIRING_RESULT)
         (is-binary WCP_FLAG)
         (is-binary EXT_FLAG)
         (is-binary HURDLE)
         (is-binary ICP)
         (is-binary NOT_ON_G2)
         (is-binary NOT_ON_G2_ACC)
         (is-binary IS_INFINITY)
         (is-binary NOT_ON_G2_ACC_MAX)
         (is-binary IS_SMALL_POINT)
         (is-binary IS_LARGE_POINT)
         (is-binary G2MTR)
         (is-binary TRIVIAL_PAIRING)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    1.3.2 Macro instruction     ;;
;;    decoding and shorthands     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (is_ecrecover)
  (+ IS_ECRECOVER_DATA IS_ECRECOVER_RESULT))

(defun (is_ecadd)
  (+ IS_ECADD_DATA IS_ECADD_RESULT))

(defun (is_ecmul)
  (+ IS_ECMUL_DATA IS_ECMUL_RESULT))

(defun (is_ecpairing)
  (+ IS_ECPAIRING_DATA IS_ECPAIRING_RESULT))

(defun (flag_sum)
  (+ (is_ecrecover) (is_ecadd) (is_ecmul) (is_ecpairing)))

;; TODO: use constants in the specs too
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
  (+ IS_ECRECOVER_DATA IS_ECADD_DATA IS_ECMUL_DATA IS_ECPAIRING_DATA))

(defun (is_result)
  (+ IS_ECRECOVER_RESULT IS_ECADD_RESULT IS_ECMUL_RESULT IS_ECPAIRING_RESULT))

(defun (transition_to_data)
  (* (- 1 (is_data)) (next (is_data))))

(defun (transition_to_result)
  (* (- 1 (is_result)) (next (is_result))))

(defun (transition_bit)
  (+ (transition_to_data) (transition_to_result)))

(defconstraint padding ()
  (if-zero STAMP
           (vanishes! (flag_sum))
           (eq! (flag_sum) 1)))

(defconstraint phase ()
  (eq! PHASE (phase_sum)))

;; In the specs this shorthand is defined in different ways in different contexts
;; Note that (transition_to_result) + (ecrecover_hypothesis) + (ecadd_hypothesis) + (ecmul_hypothesis) + (ecpairing_hypothesis) is 0 or 1
;; TODO: use the same approach in the specs
(defun (internal_checks_passed)
  (+ (* (transition_to_result) HURDLE)
     (* (ecrecover-hypothesis) (shift HURDLE INDEX_MAX_ECRECOVER_DATA))
     (* (ecadd-hypothesis) (shift HURDLE INDEX_MAX_ECADD_DATA))
     (* (ecmul-hypothesis) (shift HURDLE INDEX_MAX_ECMUL_DATA))
     (* (ecpairing-hypothesis) (shift HURDLE INDEX_MAX_ECPAIRING_DATA_MIN))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  1.3.3 constancy conditions ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancy ()
  (begin (stamp-constancy STAMP ID)
         (stamp-constancy STAMP SUCCESS_BIT)
         (stamp-constancy STAMP TOTAL_PAIRINGS)
         (stamp-constancy STAMP NOT_ON_G2_ACC_MAX)
         (stamp-constancy STAMP ICP)
         (stamp-constancy STAMP (address_sum))))

(defconstraint counter-constancy ()
  (begin (counter-constancy CT CT_MAX)
         (counter-constancy CT IS_INFINITY)
         (counter-constancy CT ACC_PAIRINGS)
         (counter-constancy CT TRIVIAL_PAIRING)
         (counter-constancy CT G2MTR)
         (counter-constancy INDEX PHASE) ;; NOTE: PHASE, NOT_ON_G2_ACC_MAX, INDEX_MAX are said to be index-constant
         (counter-constancy INDEX NOT_ON_G2_ACC_MAX)
         (counter-constancy INDEX INDEX_MAX)))

(defconstraint pair-of-points-constancy ()
  (if-not-zero ACC_PAIRINGS
               (if (will-remain-constant! ACC_PAIRINGS)
                   (will-remain-constant! ACCPC))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  1.3.4 Setting INDEX_MAX    ;;
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
;;  1.3.5 Setting TOTAL_SIZE   ;;
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
;;  1.3.6 Setting CT, CT_MAX,         ;;
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

(defconstraint set-transitions ()
  (if-not-zero (* IS_ECPAIRING_DATA (next IS_ECPAIRING_DATA))
               (if-zero (- CT CT_MAX)
                        (eq! (+ (transition_from_small_to_large) (transition_from_large_to_small)) 1))))

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
;;  1.3.7 Setting ACC_PAIRINGS ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-acc-pairings-init ()
  (if-zero IS_ECPAIRING_DATA
           (vanishes! ACC_PAIRINGS)
           (eq! (next ACC_PAIRINGS) (next IS_ECPAIRING_DATA))))

(defconstraint set-acc-pairings-increment ()
  (if-not-zero IS_ECPAIRING_DATA
               (eq! (next ACC_PAIRINGS) (+ ACC_PAIRINGS (transition_from_large_to_small)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  1.3.8 Setting ICP          ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-icp-padding ()
  (if-zero (flag_sum)
           (vanishes! ICP)))

(defconstraint set-icp ()
  (if-not-zero (transition_to_result)
               (eq! ICP (internal_checks_passed))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;  1.3.9 Setting G2MTR        ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-g2mtr ()
  (if-zero IS_LARGE_POINT
           (vanishes! G2MTR)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;  1.3.10 Setting TRIVIAL_PAIRING  ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-trivial-pairing-outside-ecpairing-data ()
  (if-zero IS_ECPAIRING_DATA
           (vanishes! TRIVIAL_PAIRING)))

;; the constraint below is equivalent:
;; (defconstraint set-trivial-pairing-init-using-expression ()
;;   (if-zero (+ (prev IS_ECPAIRING_DATA) (- 1 IS_ECPAIRING_DATA))
;;            (eq! TRIVIAL_PAIRING 1)))
(defconstraint set-trivial-pairing-init ()
  (if-zero (prev IS_ECPAIRING_DATA)
           (if-not-zero IS_ECPAIRING_DATA
                        (eq! TRIVIAL_PAIRING 1))))

(defconstraint transition-large-to-small ()
  (if-not-zero (transition_from_large_to_small)
               (eq! (next TRIVIAL_PAIRING) TRIVIAL_PAIRING)))

(defconstraint transition-small-to-large ()
  (if-not-zero (transition_from_small_to_large)
               (if-zero TRIVIAL_PAIRING
                        (vanishes! (next TRIVIAL_PAIRING))
                        (eq! (next TRIVIAL_PAIRING) (next IS_INFINITY)))))

;; note: pairing_result_hi \def LIMB_{i+1}
;; note: pairing_result_lo \def LIMB_{i+2}
(defconstraint set-pairing-result-when-trivial-pairngs ()
  (if-not-zero (transition_to_result)
               (if-not-zero ICP
                            (if-zero NOT_ON_G2_ACC_MAX
                                     (if-not-zero TRIVIAL_PAIRING
                                                  (begin (eq! SUCCESS_BIT 1)
                                                         (vanishes! (next LIMB))
                                                         (eq! (shift LIMB 2) 1)))))))

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    1.3.11 Hearbeat ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint vanishing-values ()
  (if-zero (flag_sum)
           (begin (vanishes! INDEX)
                  (vanishes! ID))))

(defconstraint stamp-increments ()
  (any! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

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
;;  1.3.13 ID increment        ;;
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
;;  1.3.14 Setting NOT_ON_G2   ;;
;;         and NOT_ON_G2_ACC   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint large-point-necessary-condition-not-on-g2 ()
  (if-not-zero IS_LARGE_POINT
               (eq! NOT_ON_G2 1)))

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
;; 1.4 Utilities       ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.4.1 WCP utilities ;;
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
;; 1.4.2 EXT utilities ;;
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
;; 1.4.3 C1 membership ;;
;;       utilities     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.4.4 Well formed   ;;
;;       coordinates   ;;
;;       utilities     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.6 Specialized     ;;
;;     constraints     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.6.1 The ECRECOVER ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecrecover-hypothesis)
  (* IS_ECRECOVER_DATA
     (~ (- ID (prev ID)))))

(defun (h_hi)
  LIMB)

(defun (h_lo)
  (next LIMB))

(defun (v_hi)
  (shift LIMB 2))

(defun (v_lo)
  (shift LIMB 3))

(defun (r_hi)
  (shift LIMB 4))

(defun (r_lo)
  (shift LIMB 5))

(defun (s_hi)
  (shift LIMB 6))

(defun (s_lo)
  (shift LIMB 7))

(defun (r_is_in_range)
  WCP_RES)

(defun (r_is_positive)
  (next WCP_RES))

(defun (s_is_in_range)
  (shift WCP_RES 2))

(defun (s_is_positive)
  (shift WCP_RES 3))

(defun (v_is_27)
  (shift WCP_RES 4))

(defun (v_is_28)
  (shift WCP_RES 5))

(defconstraint internal-checks-ecrecover (:guard (ecrecover-hypothesis))
  (begin (callToLT 0 (r_hi) (r_lo) SECP256K1N_HI SECP256K1N_LO)
         (callToLT 1 0 0 (r_hi) (r_lo))
         (callToLT 2 (s_hi) (s_lo) SECP256K1N_HI SECP256K1N_LO)
         (callToLT 3 0 0 (s_hi) (s_lo))
         (callToEQ 4 (v_hi) (v_lo) 0 27)
         (callToEQ 5 (v_hi) (v_lo) 0 28)))

(defconstraint justify-success-bit-ecrecover (:guard (ecrecover-hypothesis))
  (begin (eq! HURDLE (* (r_is_in_range) (r_is_positive)))
         (eq! (next HURDLE) (* (s_is_in_range) (s_is_positive)))
         (eq! (shift HURDLE 2)
              (* HURDLE (next HURDLE)))
         (eq! (internal_checks_passed)
              (* (shift HURDLE 2) (+ (v_is_27) (v_is_28))))
         (if-zero (internal_checks_passed)
                  (vanishes! SUCCESS_BIT))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.6.2 The ECADD     ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecadd-hypothesis)
  (* IS_ECADD_DATA
     (~ (- ID (prev ID)))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.6.3 The ECMUL     ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecmul-hypothesis)
  (* IS_ECMUL_DATA
     (~ (- ID (prev ID)))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.6.4 The ECPAIRING ;;
;;       case          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ecpairing-hypothesis)
  (* IS_ECPAIRING_DATA
     (- ACC_PAIRINGS (prev ACC_PAIRINGS))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.7 Elliptic curve  ;;
;;      circuit flags  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 1.7.3 Interface for ;;
;;       Gnark         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint ecrecover-circuit-selector ()
  (eq! CS_ECRECOVER (* ICP (is_ecrecover))))

(defconstraint ecadd-circuit-selector ()
  (eq! CS_ECADD (* ICP (is_ecadd))))

(defconstraint ecmul-circuit-selector ()
  (eq! CS_ECMUL (* ICP (is_ecmul))))

(defconstraint ecpairing-circuit-selector ()
  (eq! CS_ECPAIRING ACCPC))

(defconstraint g2-membership-circuit-selector ()
  (eq! CS_G2_MEMBERSHIP G2MTR))

;; ;;;;;;;;;;;;;;;;;;;;;;;;
;; ;;                    ;;
;; ;;    4 Lookups       ;;
;; ;;                    ;;
;; ;;;;;;;;;;;;;;;;;;;;;;;;
;; (defun (wcp-lookup _shift arg1_hi arg1_lo arg2_hi arg2_lo inst res)
;;   (begin (= (shift WCP_ARG1_HI _shift) arg1_hi)
;;          (= (shift WCP_ARG1_LO _shift) arg1_lo)
;;          (= (shift WCP_ARG2_HI _shift) arg2_hi)
;;          (= (shift WCP_ARG2_LO _shift) arg2_lo)
;;          (= (shift WCP_INST _shift) inst)
;;          (= (shift WCP_RES _shift) res)))
;; (defun (ext-lookup _shift arg1_hi arg1_lo arg2_hi arg2_lo arg3_hi arg3_lo inst res_hi res_lo)
;;   (begin (= (shift EXT_ARG1_HI _shift) arg1_hi)
;;          (= (shift EXT_ARG1_LO _shift) arg1_lo)
;;          (= (shift EXT_ARG2_HI _shift) arg2_hi)
;;          (= (shift EXT_ARG2_LO _shift) arg2_lo)
;;          (= (shift EXT_ARG3_HI _shift) arg3_hi)
;;          (= (shift EXT_ARG3_LO _shift) arg3_lo)
;;          (= (shift EXT_INST _shift) inst)
;;          (= (shift EXT_RES_HI _shift) res_hi)
;;          (= (shift EXT_RES_LO _shift) res_lo)))
;; (defun (check-c1-membership)
;;   ;; u = 0 for the first point of C1, and u = 1 for the second point (ecAdd only)
;;   (begin ;; --------------------- WCP lookup ---------------------
;;          ;; Comparison of x and y with p
;;          (for v
;;               [1]                                       ;; v = 0 for x, v = 1 for y
;;               (wcp-lookup v                             ;; shift
;;                           (shift LIMB (* 2 v))          ;; arg 1 high
;;                           (shift LIMB
;;                                  (+ (* 2 v) 1))         ;; arg 1 low
;;                           P_HI                          ;; arg 2 high
;;                           P_LO                          ;; arg 2 low
;;                           OPCODE_LT                     ;; instruction
;;                           (shift COMPARISONS (* 2 v)))) ;; result
;;          ;; Comparison of y^2 with x^3 + 3
;;          (wcp-lookup 2                                  ;; shift
;;                      (shift SQUARE 2)                   ;; arg 1 high
;;                      (shift SQUARE 3)                   ;; arg 1 low
;;                      (shift CUBE 2)                     ;; arg 2 high
;;                      (shift CUBE 3)                     ;; arg 2 low
;;                      OPCODE_EQ                          ;; instruction
;;                      (shift EQUALITIES 1))
;;          ;; --------------------- EXT lookup ---------------------
;;          ;; x^2, y^2 mod p
;;          (for v
;;               [1]                                       ;; v = 0 for x, v = 1 for y
;;               (ext-lookup v                             ;; shift
;;                           (shift LIMB (* 2 v))          ;; arg1 high
;;                           (shift LIMB
;;                                  (+ (* 2 v) 1))         ;; arg1 low
;;                           (shift LIMB (* 2 v))          ;; arg2 hi
;;                           (shift LIMB
;;                                  (+ (* 2 v) 1))         ;; arg2 low
;;                           P_HI                          ;; arg3 high
;;                           P_LO                          ;; arg3 low
;;                           OPCODE_MULMOD                 ;; instruction
;;                           (shift SQUARE (* 2 v))        ;; res high
;;                           (shift SQUARE
;;                                  (+ 1 (* 2 v)))))       ;; res low
;;          ;; x^3 mod p
;;          (ext-lookup 2                                  ;; shift
;;                      (shift SQUARE 0)                   ;; arg1 high
;;                      (shift SQUARE 1)                   ;; arg1 low
;;                      (shift LIMB 0)                     ;; arg2 high
;;                      (shift LIMB 1)                     ;; arg2 low
;;                      P_HI                               ;; arg3 high
;;                      P_LO                               ;; arg3 low
;;                      OPCODE_MULMOD                      ;; instruction
;;                      (shift CUBE 0)                     ;; res high
;;                      (shift CUBE 1))                    ;; res low
;;          ;; x^3 + 3 mod p
;;          (ext-lookup 3                                  ;; shift
;;                      (shift CUBE 0)                     ;; arg1 high
;;                      (shift CUBE 1)                     ;; arg1 low
;;                      0                                  ;; arg2 high
;;                      3                                  ;; arg2 low
;;                      P_HI                               ;; arg3 high
;;                      P_LO                               ;; arg3 low
;;                      OPCODE_ADDMOD                      ;; instruction
;;                      (shift CUBE 2)                     ;; res high
;;                      (shift CUBE 3))))                  ;; res low
;; ;; 4.1
;; (defconstraint c1-membership ()
;;   (if-not-zero (any ;; 1 if STAMP[i-1] != STAMP[i] and [EC_MUL = 1 or EC_PAIRING = 1], else 0
;;                     (and (is-not-zero (- (prev STAMP) STAMP))
;;                          (+ EC_MUL EC_PAIRING))
;;                     ;; 1 if we are seeing a new pairing at row i in a call to ecPairing (potentially not including the first one,
;;                     ;; which is captured by the condition above)
;;                     (and EC_PAIRING
;;                          (- (prev ACC_PAIRINGS) ACC_PAIRINGS))
;;                     ;; 1 if CT_MIN[i] = 0 and EC_ADD[i] = 1, else 0
;;                     (and (is-zero CT_MIN) EC_ADD))
;;                ;; if any of the 3 condition above is true, we need to justify (or refute) the membership of a point to C1
;;                (check-c1-membership)))
;; ;; 4.2
;; (defconstraint lookup-ecpairing-wcp ()
;;   (if-eq EC_PAIRING 1
;;          (if-zero INDEX
;;                   ;; Comparison of Im(a), Re(a), Im(b), Re(b) with p
;;                   (for v
;;                        [3]
;;                        (wcp-lookup (+ 3 v)                    ;; shift
;;                                    (shift LIMB
;;                                           (+ (* 2 v) 4))      ;; arg 1 high
;;                                    (shift LIMB
;;                                           (+ (* 2 v) 5))      ;; arg 1 low
;;                                    P_HI                       ;; arg 2 high
;;                                    P_LO                       ;; arg 2 low
;;                                    OPCODE_LT                  ;; instruction
;;                                    (shift COMPARISONS
;;                                           (+ (* 2 v) 4))))))) ;; result
;; ;; 4.2
;; (defconstraint lookup-ecrecover-wcp ()
;;   (if-eq EC_RECOVER 1
;;          (if-zero INDEX
;;                   (begin ;; Comparison of r and s with secp256k1n
;;                          (for u
;;                               [1]
;;                               (wcp-lookup u                                ;; shift
;;                                           (shift LIMB
;;                                                  (+ (* 2 u) 4))            ;; arg 1 high
;;                                           (shift LIMB
;;                                                  (+ (* 2 u) 5))            ;; arg 1 low
;;                                           SECP256K1N_HI                    ;; arg 2 high
;;                                           SECP256K1N_LO                    ;; arg 2 low
;;                                           OPCODE_LT                        ;; instruction
;;                                           (shift COMPARISONS (* 4 u))))    ;; result
;;                          ;; Comparison of r and s with secp256k1n
;;                          (for u
;;                               [1]
;;                               (wcp-lookup (+ u 2)                          ;; shift
;;                                           0                                ;; arg 1 high
;;                                           0                                ;; arg 1 low
;;                                           (shift LIMB
;;                                                  (+ (* 2 u) 4))            ;; arg 2 high
;;                                           (shift LIMB
;;                                                  (+ (* 2 u) 5))            ;; arg 2 low
;;                                           OPCODE_LT                        ;; instruction
;;                                           (shift COMPARISONS
;;                                                  (+ (* 4 u) 2))))          ;; result
;;                          ;; Comparison of v with 27 and 28
;;                          (for u
;;                               [1]
;;                               (wcp-lookup (+ u 4)                          ;; shift
;;                                           (shift LIMB 2)                   ;; arg 1 high
;;                                           (shift LIMB 3)                   ;; arg 1 low
;;                                           0                                ;; arg 2 high
;;                                           (+ 27 u)                         ;; arg 2 low
;;                                           OPCODE_EQ                        ;; instruction
;;                                           (shift EQUALITIES (+ 1 u)))))))) ;; result


