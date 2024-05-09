(module ecdata)

(defconst
  P_HI            0x30644e72e131a029b85045b68181585d
  P_LO            0x97816a916871ca8d3c208c16d87cfd47
  SECP256K1N_HI   0xffffffffffffffffffffffffffffffff
  SECP256K1N_LO   0xfffffffffffffffffffffffefffffc2f
  OPCODE_LT       0x10
  OPCODE_EQ       0x14
  OPCODE_MULMOD   0x9
  OPCODE_ADDMOD   0x8)

(defpurefun (if-not-eq X Y Z)
  (if-not-zero (- X Y) Z))

(defunalias if-zero-else if-zero)
(defunalias doesnt-vanish is-zero)

(defpurefun (differ X Y)
  (doesnt-vanish (- X Y)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.2 Constancy conditions    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (stamp-consitency X)
    (if-eq (next STAMP) STAMP (will-remain-constant! X)))

;; 3.2.1
(defconstraint stamp-constancies ()
  (begin
   (stamp-consitency TYPE)
   (stamp-consitency PCP)
   (stamp-consitency PRELIMINARY_CHECKS_PASSED)
   (stamp-consitency SOMETHING_WASNT_ON_G2)
   (stamp-consitency TOTAL_PAIRINGS)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    3.3 Type conditions    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.3.1
(defconstraint exactly-one-type ()
  (if-not-zero STAMP
               (eq! (+ EC_RECOVER EC_ADD EC_MUL EC_PAIRING) 1)))

;; 3.3.2
(defconstraint type-consistency ()
  (begin
   (if-eq EC_RECOVER 1 (= TYPE 1))
   (if-eq EC_ADD 1 (= TYPE 6))
   (if-eq EC_MUL 1 (= TYPE 7))
   (if-eq EC_PAIRING 1 (= TYPE 8))))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    3.4 Monotony    ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.4.1
(defconstraint hurdle-non-increasing ()
  (if-eq (next STAMP) STAMP
         (if-not-eq (next HURDLE) HURDLE
                    (= (next HURDLE) (- HURDLE 1)))))

;; 3.4.2
(defconstraint notOnG2Acc-non-decreasing ()
  (if-eq (next STAMP) STAMP
         (if-not-eq (next THIS_IS_NOT_ON_G2_ACC) THIS_IS_NOT_ON_G2_ACC
                    (= (next THIS_IS_NOT_ON_G2_ACC) (+ THIS_IS_NOT_ON_G2_ACC 1)))))

;; 3.4.3
(defconstraint notOnG2-non-decreasing ()
  (if-not-zero (next INDEX)
               (if-not-eq (next THIS_IS_NOT_ON_G2) THIS_IS_NOT_ON_G2
                          (= (next THIS_IS_NOT_ON_G2) (+ THIS_IS_NOT_ON_G2 1)))))


;; 3.4.4
(defconstraint notOnG2-restarts-zero ()
  (if-zero INDEX (vanishes! THIS_IS_NOT_ON_G2)))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    3.5 Hearbeat    ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.5.1)
(defconstraint first-row (:domain {0}) (vanishes! STAMP))

;; 3.5.2)
(defconstraint everything-vanish-initially ()
  (if-zero STAMP (begin
                  (vanishes! INDEX)
                  (vanishes! TYPE)
                  (vanishes! (+ EC_RECOVER EC_ADD EC_MUL EC_PAIRING PCP PRELIMINARY_CHECKS_PASSED SOMETHING_WASNT_ON_G2)))))

;; 3.5.3)
(defconstraint first-index-vanishes! ()
  (if-zero STAMP (vanishes! (next INDEX))))

;; 3.5.4)
(defconstraint ct-min-heartbeat (:guard STAMP)
  (if-eq-else (next STAMP) STAMP
              (if-eq-else CT_MIN 3
                          (vanishes! (next CT_MIN))
                          (= (next CT_MIN) (+ CT_MIN 1)))
              (vanishes! (next CT_MIN))))

;; 3.5.5)
(defconstraint index-heartbeat ()
  (begin
   (if-eq EC_PAIRING 1
          (if-eq-else INDEX 11
                      (vanishes! (next INDEX))
                      (= (next INDEX) (+ INDEX 1))))
   (if-eq (+ EC_ADD EC_RECOVER) 1
          (if-eq-else INDEX 7
                      (vanishes! (next INDEX))
                      (= (next INDEX) (+ INDEX 1))))
   (if-eq EC_MUL 1
          (if-eq-else INDEX 5
                      (vanishes! (next INDEX))
                      (= (next INDEX) (+ INDEX 1))))))

;; 3.5.6)
(defconstraint stamp-behaviour ()
  (if-not-zero (next STAMP)
               (if-zero-else (next INDEX)
                             (if-eq-else EC_PAIRING 1
                                         (if-eq-else TOTAL_PAIRINGS (+ ACC_PAIRINGS 1)
                                                     (will-change! STAMP)
                                                     (will-remain-constant! STAMP))
                                         (will-change! STAMP))
                             (will-remain-constant! STAMP))))

;; 3.5.7)
(defconstraint acc-pairings-behaviour ()
  (if-eq-else (next STAMP) STAMP
              (if-eq-else INDEX 11
                          (= (next ACC_PAIRINGS) (+ ACC_PAIRINGS 1))
                          (= (next ACC_PAIRINGS) ACC_PAIRINGS))
              (vanishes! (next ACC_PAIRINGS))))

;; 3.5.8)
(defconstraint finalization-constraints (:domain {-1})
  (begin
   (if-eq EC_PAIRING 1
          (begin
           (= INDEX 11)
           (= TOTAL_PAIRINGS (+ ACC_PAIRINGS 1))))
   (if-eq (+ EC_ADD EC_RECOVER) 1
          (= INDEX 7))
   (if-eq EC_MUL 1
          (= INDEX 5))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    3.6 Byte decompositions    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.6.1
(defconstraint byte-decompositions ()
  (if-eq-else (next STAMP) STAMP
              (= (next ACC_DELTA) (+ (* 256 ACC_DELTA) (next BYTE_DELTA)))
              (= (next ACC_DELTA) (next BYTE_DELTA))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    3.7 Connection constraints    ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.7.1
(defconstraint connection-constraints ()
  (if-not-eq (next STAMP) STAMP
             (= (next STAMP) (+ STAMP 1 (shift ACC_DELTA 4)))))


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;    3.8 Hurdle    ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;


;; 3.8.1.a
(defconstraint final-hurdle-is-passed-to-pcp ()
  (if-not-eq (next STAMP) STAMP
             (= PCP HURDLE)))

;; 3.8.1.b
(defconstraint final-pcp (:domain {-1})
  (= PCP HURDLE))

;; 3.8.2
(defconstraint initial-hurdle ()
  (if-not-eq (next STAMP) STAMP
             (= (next HURDLE) (next COMPARISONS))))

;; 3.8.3
(defconstraint hurdle-behaviour ()
  (if-eq (next STAMP) STAMP
         (begin
          (if-eq (next CT_MIN) 1 (= (next HURDLE) HURDLE))
          (if-eq (next CT_MIN) 3 (= (next HURDLE) (* HURDLE (next EQUALITIES))))
          (if-not-eq (next CT_MIN) 1 (if-not-eq (next CT_MIN) 3 (= (next HURDLE) (* HURDLE (next COMPARISONS))))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.9 Comaprisons    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.9.1
(defconstraint hardcodec-comparison ()
  (if-eq EC_MUL 1
         (if-eq INDEX 4
                (= COMPARISONS 1))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.10 Equalities    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.10.1
(defconstraint first-equalities ()
  (if-eq (shift INDEX 2) 3
         (= (shift EQUALITIES 2) (+ (next EQUALITIES) EQUALITIES))))

;; 3.10.2
(defconstraint middle-equalities ()
  (if-eq INDEX 7
         (begin
          (if-eq (+ EC_PAIRING EC_RECOVER) 1 (= EQUALITIES 1))
          (if-eq EC_ADD 1 (= EQUALITIES (+ (prev EQUALITIES) (shift EQUALITIES -2)))))))

;; 3.10.3
(defconstraint last-equalities ()
  (if-eq INDEX 11 (= EQUALITIES 1)))

;; 3.10.4.a
(defconstraint point-infinity-a ()
  (if-eq (+ EC_MUL EC_PAIRING) 1
         (if-zero INDEX
                  (if-zero-else (+ LIMB (next LIMB) (shift LIMB 2) (shift LIMB 3))
                                (= (shift EQUALITIES 2) 1)
                                (vanishes! (shift EQUALITIES 2))))))

;; 3.10.4.b
(defconstraint point-infinity-b ()
  (if-eq EC_ADD 1
         (if-zero CT_MIN
                  (if-zero-else (+ LIMB (next LIMB) (shift LIMB 2) (shift LIMB 3))
                                (= (shift EQUALITIES 2) 1)
                                (vanishes! (shift EQUALITIES 2))))))


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;    3.11 Gnark    ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;


;; 3.11.1
(defconstraint initial-not-on-g2-acc ()
  (if-not-eq (next STAMP) STAMP
             (vanishes! (next THIS_IS_NOT_ON_G2_ACC))))

;; 3.11.2
(defconstraint not-on-g2-acc-activation-condition ()
  (if-eq (next THIS_IS_NOT_ON_G2_ACC) (+ THIS_IS_NOT_ON_G2_ACC 1)
         (begin
          (= (next INDEX) 11)
          (= (next THIS_IS_NOT_ON_G2) 1))))

;; 3.11.3
(defconstraint not-on-g2-should-trigger-acc ()
  (if-eq THIS_IS_NOT_ON_G2 1
         (if-eq INDEX 11
                (= THIS_IS_NOT_ON_G2_ACC 1))))

;; 3.11.4
(defconstraint not-on-g2-triggering-condition ()
  (if-eq (next THIS_IS_NOT_ON_G2) (+ THIS_IS_NOT_ON_G2 1)
         (begin
          (= (next EC_PAIRING) 1)
          (= (next PCP)  1)
          (= (next INDEX) 4)
          (= (next THIS_IS_NOT_ON_G2_ACC) 0))))

;; 3.11.5
(defconstraint not-on-g2-acc-final-value ()
  (if-not-eq (next STAMP) STAMP
             (= SOMETHING_WASNT_ON_G2 THIS_IS_NOT_ON_G2_ACC)))

;; 3.11.5-bis
(defconstraint not-on-g2-acc-final-value-if-last-row (:domain {-1})
  (= SOMETHING_WASNT_ON_G2 THIS_IS_NOT_ON_G2_ACC))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    3.12 AllChecksPassed       ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; 3.12.1
(defconstraint all-checks-passed-not-pairing()
  (if-not-zero (+ EC_RECOVER EC_ADD EC_MUL)
      (= ALL_CHECKS_PASSED PRELIMINARY_CHECKS_PASSED)))

;; 3.12.1
(defconstraint all-checks-passed-pairing()
  (if-not-zero EC_PAIRING
      (= ALL_CHECKS_PASSED (* PRELIMINARY_CHECKS_PASSED (- 1 SOMETHING_WASNT_ON_G2)))))

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    4 Lookups       ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


(defun (wcp-lookup _shift arg1_hi arg1_lo arg2_hi arg2_lo inst res)
    (begin
     (= (shift WCP_ARG1_HI _shift) arg1_hi)
     (= (shift WCP_ARG1_LO _shift) arg1_lo)
     (= (shift WCP_ARG2_HI _shift) arg2_hi)
     (= (shift WCP_ARG2_LO _shift) arg2_lo)
     (= (shift WCP_INST _shift) inst)
     (= (shift WCP_RES _shift) res)))

(defun (ext-lookup _shift arg1_hi arg1_lo arg2_hi arg2_lo arg3_hi arg3_lo inst res_hi res_lo)
    (begin
     (= (shift EXT_ARG1_HI _shift) arg1_hi)
     (= (shift EXT_ARG1_LO _shift) arg1_lo)
     (= (shift EXT_ARG2_HI _shift) arg2_hi)
     (= (shift EXT_ARG2_LO _shift) arg2_lo)
     (= (shift EXT_ARG3_HI _shift) arg3_hi)
     (= (shift EXT_ARG3_LO _shift) arg3_lo)
     (= (shift EXT_INST _shift) inst)
     (= (shift EXT_RES_HI _shift) res_hi)
     (= (shift EXT_RES_LO _shift) res_lo)))

(defun (check-c1-membership)
  ;; u = 0 for the first point of C1, and u = 1 for the second point (ecAdd only)
  (begin
    ;; --------------------- WCP lookup ---------------------

    ;; Comparison of x and y with p
    (for v [1] ;; v = 0 for x, v = 1 for y
      (wcp-lookup
        v ;; shift
        (shift LIMB (* 2 v)) ;; arg 1 high
        (shift LIMB (+ (* 2 v) 1)) ;; arg 1 low
        P_HI ;; arg 2 high
        P_LO ;; arg 2 low
        OPCODE_LT ;; instruction
        (shift COMPARISONS (* 2 v)))) ;; result

    ;; Comparison of y^2 with x^3 + 3
    (wcp-lookup
      2 ;; shift
      (shift SQUARE 2) ;; arg 1 high
      (shift SQUARE 3) ;; arg 1 low
      (shift CUBE 2) ;; arg 2 high
      (shift CUBE 3) ;; arg 2 low
      OPCODE_EQ ;; instruction
      (shift EQUALITIES 1))

      ;; --------------------- EXT lookup ---------------------

      ;; x^2, y^2 mod p
      (for v [1] ;; v = 0 for x, v = 1 for y
        (ext-lookup
          v ;; shift
          (shift LIMB (* 2 v)) ;; arg1 high
          (shift LIMB (+ (* 2 v) 1)) ;; arg1 low
          (shift LIMB (* 2 v)) ;; arg2 hi
          (shift LIMB (+ (* 2 v) 1)) ;; arg2 low
          P_HI ;; arg3 high
          P_LO ;; arg3 low
          OPCODE_MULMOD ;; instruction
          (shift SQUARE (* 2 v)) ;; res high
          (shift SQUARE (+ 1 (* 2 v))))) ;; res low

      ;; x^3 mod p
      (ext-lookup
        2 ;; shift
        (shift SQUARE 0) ;; arg1 high
        (shift SQUARE 1) ;; arg1 low
        (shift LIMB 0) ;; arg2 high
        (shift LIMB 1) ;; arg2 low
        P_HI ;; arg3 high
        P_LO ;; arg3 low
        OPCODE_MULMOD ;; instruction
        (shift CUBE 0) ;; res high
        (shift CUBE 1)) ;; res low

      ;; x^3 + 3 mod p
      (ext-lookup
        3 ;; shift
        (shift CUBE 0) ;; arg1 high
        (shift CUBE 1) ;; arg1 low
        0 ;; arg2 high
        3 ;; arg2 low
        P_HI ;; arg3 high
        P_LO ;; arg3 low
        OPCODE_ADDMOD ;; instruction
        (shift CUBE 2) ;; res high
        (shift CUBE 3)))) ;; res low

;; 4.1
(defconstraint c1-membership ()
  (if-not-zero
      (any
        ;; 1 if STAMP[i-1] != STAMP[i] and [EC_MUL = 1 or EC_PAIRING = 1], else 0
        (and
          (is-not-zero (- (prev STAMP) STAMP))
          (+ EC_MUL EC_PAIRING))
        ;; 1 if we are seeing a new pairing at row i in a call to ecPairing (potentially not including the first one,
        ;; which is captured by the condition above)
        (and
          EC_PAIRING
          (- (prev ACC_PAIRINGS) ACC_PAIRINGS))
        ;; 1 if CT_MIN[i] = 0 and EC_ADD[i] = 1, else 0
        (and
          (is-zero CT_MIN)
          EC_ADD))

      ;; if any of the 3 condition above is true, we need to justify (or refute) the membership of a point to C1
      (check-c1-membership)))

;; 4.2
(defconstraint lookup-ecpairing-wcp ()
  (if-eq EC_PAIRING 1
         (if-zero INDEX
                  ;; Comparison of Im(a), Re(a), Im(b), Re(b) with p
                  (for v [3]
                       (wcp-lookup
                        (+ 3 v) ;; shift
                        (shift LIMB (+ (* 2 v) 4)) ;; arg 1 high
                        (shift LIMB (+ (* 2 v) 5)) ;; arg 1 low
                        P_HI ;; arg 2 high
                        P_LO ;; arg 2 low
                        OPCODE_LT ;; instruction
                        (shift COMPARISONS (+ (* 2 v) 4))))))) ;; result

;; 4.2
(defconstraint lookup-ecrecover-wcp ()
  (if-eq EC_RECOVER 1
         (if-zero INDEX
                  (begin
                   ;; Comparison of r and s with secp256k1n
                   (for u [1]
                        (wcp-lookup
                         u ;; shift
                         (shift LIMB (+ (* 2 u) 4)) ;; arg 1 high
                         (shift LIMB (+ (* 2 u) 5)) ;; arg 1 low
                         SECP256K1N_HI ;; arg 2 high
                         SECP256K1N_LO ;; arg 2 low
                         OPCODE_LT ;; instruction
                         (shift COMPARISONS (* 4 u)))) ;; result

                   ;; Comparison of r and s with secp256k1n
                   (for u [1]
                        (wcp-lookup
                         (+ u 2) ;; shift
                         0 ;; arg 1 high
                         0 ;; arg 1 low
                         (shift LIMB (+ (* 2 u) 4)) ;; arg 2 high
                         (shift LIMB (+ (* 2 u) 5)) ;; arg 2 low
                         OPCODE_LT ;; instruction
                         (shift COMPARISONS (+ (* 4 u) 2)))) ;; result

                   ;; Comparison of v with 27 and 28
                   (for u [1]
                        (wcp-lookup
                         (+ u 4) ;; shift
                         (shift LIMB 2) ;; arg 1 high
                         (shift LIMB 3) ;; arg 1 low
                         0 ;; arg 2 high
                         (+ 27 u) ;; arg 2 low
                         OPCODE_EQ ;; instruction
                         (shift EQUALITIES (+ 1 u)))))))) ;; result
