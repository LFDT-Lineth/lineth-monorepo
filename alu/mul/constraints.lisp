(module mul)

(defconst
  ONETWOEIGHT 128
  ONETWOSEVEN 127
  THETA       18446744073709551616                     ;18446744073709551616 = 256^8
  THETA2      340282366920938463463374607431768211456) ;340282366920938463463374607431768211456 = 256^16

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    1.3 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint stamp-init (:domain {0}) ;; ""
  (vanishes! STAMP))

(defconstraint stamp-update ()
  (vanishes! (* (will-inc! STAMP 1) (will-remain-constant! STAMP))))

(defconstraint vanishing ()
  (if-zero STAMP
           (begin (vanishes! CT)
                  (vanishes! OLI)
                  (vanishes! INST))))

(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP ARG_1_HI)
         (stamp-constancy STAMP ARG_1_LO)
         (stamp-constancy STAMP ARG_2_HI)
         (stamp-constancy STAMP ARG_2_LO)
         (stamp-constancy STAMP RES_HI)
         (stamp-constancy STAMP RES_LO)
         (stamp-constancy STAMP INST)))

(defconstraint instruction-constraining ()
  (if-not-zero STAMP
               (vanishes! (* (- INST EVM_INST_MUL) (- INST EVM_INST_EXP)))))

(defconstraint reset-stuff ()
  (if-not-zero (will-remain-constant! STAMP)
               (begin (vanishes! (next CT))
                      (vanishes! (next BIT_NUM)))))

(defconstraint oli-last-one-line ()
  (if-not-zero OLI
               (will-inc! STAMP 1)))

(defconstraint counter-update (:guard STAMP)
  (if-zero OLI
           (if-not-zero (- CT MMEDIUMMO)
                        (will-inc! CT 1))))

(defconstraint counter-reset ()
  (if-eq CT MMEDIUMMO
         (vanishes! (next CT))))

(defconstraint counter-constancies ()
  (begin (counter-constancy CT SNM)
         (counter-constancy CT BIT_NUM)
         (counter-constancy CT ESRC)
         (counter-constancy CT EBIT)
         (counter-constancy CT EACC)))

(defconstraint other-resets ()
  (if-eq CT MMEDIUMMO
         (begin (if-not-zero (- INST EVM_INST_EXP)
                             (will-inc! STAMP 1)) ; i.e. INST == MUL
                (if-not-zero (- INST EVM_INST_MUL)         ; i.e. INST == EXP
                             (if-eq RESV 1 (will-inc! STAMP 1))))))

(defconstraint bit_num-doesnt-reach-oneTwoEight ()
  (if-eq BIT_NUM ONETWOEIGHT (vanishes! 1)))

(defconstraint last-row (:domain {-1} :guard STAMP) ;; ""
  (begin (debug (= OLI 1))
         (= INST EVM_INST_EXP)
         (vanishes! ARG_1_HI)
         (vanishes! ARG_1_LO)
         (vanishes! ARG_2_HI)
         (vanishes! ARG_2_LO)
         (debug (eq! RES_HI 0))
         (debug (eq! RES_LO 1))))

(defun (first-row)
  (if-not-zero (- (prev STAMP) STAMP)
               (begin (= SNM 1)
                      (= EBIT 1)
                      (= EACC 1)
                      (if-zero ARG_2_HI
                               (= ESRC 1)
                               (vanishes! ESRC)))))

;; exponent-bit-source-is-high-limb applies when ESRC == 0
(defun (exponent-bit-source-is-high-limb)
  (begin (will-remain-constant! STAMP)
         (vanishes! (next SNM))
         (if-eq-else EACC ARG_2_HI
                     (begin (will-eq! ESRC 1)
                            (vanishes! (next EACC))
                            (vanishes! (next BIT_NUM)))
                     (begin (vanishes! (next ESRC))
                            (will-eq! EACC (* 2 EACC))
                            (will-inc! BIT_NUM 1)))))

;; exponent-bit-source-is-low-limb applies when ESRC == 1
(defun (exponent-bit-source-is-low-limb)
  (if-not-zero ARG_2_HI
               (if-eq-else BIT_NUM ONETWOSEVEN
                           ;; (ARG_2_HI != 0) et (BIT_NUM == 127)
                           (begin (will-inc! STAMP 1)
                                  (= EACC ARG_2_LO))
                           ;; (ARG_2_HI != 0) et (BIT_NUM != 127)
                           (begin (vanishes! (next SNM))
                                  (will-remain-constant! STAMP)
                                  (will-eq! ESRC 1)
                                  (will-eq! EACC (* 2 EACC))
                                  (will-inc! BIT_NUM 1)))
               (if-eq-else EACC ARG_2_LO
                           ;; (ARG_2_HI == 0) et (EACC == ARG_2_LO)
                           (will-inc! STAMP 1)
                           ;; (ARG_2_HI == 0) et (EACC != ARG_2_LO)
                           (begin (vanishes! (next SNM))
                                  (will-remain-constant! STAMP)
                                  (will-eq! ESRC 1)
                                  (will-eq! EACC (* 2 EACC))
                                  (will-inc! BIT_NUM 1)))))

(defun (end-of-cycle)
  (if-zero (- SNM EBIT)
           ;; SNM == EBIT
           (if-zero ESRC
                    ;; ESRC == 0 i.e. source = high part
                    (exponent-bit-source-is-high-limb)
                    ;; ESRC == 1 i.e. source = low part
                    (exponent-bit-source-is-low-limb))
           ;; SNM != EBIT
           (begin (will-remain-constant! STAMP)
                  (will-inc! SNM 1)
                  (will-remain-constant! EBIT)
                  (will-inc! EACC 1)
                  (will-remain-constant! ESRC)
                  (will-remain-constant! BIT_NUM))))

(defconstraint nontrivial-exp-regime-nonzero-result-heartbeat ()
  (if-eq INST EVM_INST_EXP
         (if-zero OLI
                  (if-zero RESV
                           (begin (first-row)
                                  (if-eq CT MMEDIUMMO (end-of-cycle)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    1.5 byte decompositions    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint byte-decompositions ()
  (begin (byte-decomposition CT ACC_A_0 BYTE_A_0)
         (byte-decomposition CT ACC_A_1 BYTE_A_1)
         (byte-decomposition CT ACC_A_2 BYTE_A_2)
         (byte-decomposition CT ACC_A_3 BYTE_A_3)
         ;
         (byte-decomposition CT ACC_B_0 BYTE_B_0)
         (byte-decomposition CT ACC_B_1 BYTE_B_1)
         (byte-decomposition CT ACC_B_2 BYTE_B_2)
         (byte-decomposition CT ACC_B_3 BYTE_B_3)
         ;
         (byte-decomposition CT ACC_C_0 BYTE_C_0)
         (byte-decomposition CT ACC_C_1 BYTE_C_1)
         (byte-decomposition CT ACC_C_2 BYTE_C_2)
         (byte-decomposition CT ACC_C_3 BYTE_C_3)
         ;
         (byte-decomposition CT ACC_H_0 BYTE_H_0)
         (byte-decomposition CT ACC_H_1 BYTE_H_1)
         (byte-decomposition CT ACC_H_2 BYTE_H_2)
         (byte-decomposition CT ACC_H_3 BYTE_H_3)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;    1.6 TINYB, TINYE, OLI and RESV constraints    ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint tiny-base (:guard STAMP)
  (if-not-zero ARG_1_HI
               (vanishes! TINYB)
               (if-not-zero (* ARG_1_LO (- 1 ARG_1_LO))
                            (vanishes! TINYB)
                            (= TINYB 1))))

(defconstraint tiny-exponent (:guard STAMP)
  (if-not-zero ARG_2_HI
               (vanishes! TINYE)
               (if-not-zero (* ARG_2_LO (- 1 ARG_2_LO))
                            (vanishes! TINYE)
                            (= TINYE 1))))

(defconstraint result-vanishes! (:guard STAMP)
  (if-not-zero RES_HI
               (vanishes! RESV)
               (if-not-zero RES_LO
                            (vanishes! RESV)
                            (= RESV 1))))

(defconstraint one-line-instruction (:guard STAMP)
  (= (+ OLI (* TINYB TINYE))
     (+ TINYB TINYE)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    1.7 trivial regime    ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint trivial-regime ()
  (if-eq OLI 1
         ;; since OLI != 0 we have STAMP != 0
         ;; thus INST ∈ {MUL, EXP}
         (begin (if-not-zero (- INST EVM_INST_EXP)
                             ;; i.e. INST == MUL
                             (begin (if-eq TINYE 1
                                           ;; i.e. ARG_2 = ARG_2_LO ∈ {0, 1}
                                           (begin (= RES_HI (* ARG_2_LO ARG_1_HI))
                                                  (= RES_LO (* ARG_2_LO ARG_1_LO))))
                                    (if-eq TINYB 1
                                           ;; i.e. ARG_1 = ARG_1_LO ∈ {0, 1}
                                           (begin (= RES_HI (* ARG_1_LO ARG_2_HI))
                                                  (= RES_LO (* ARG_1_LO ARG_2_LO))))))
                (if-not-zero (- INST EVM_INST_MUL)
                             ;; i.e. INST == EXP
                             (begin (if-eq-else TINYE 1
                                                ;; TINYE == 1 <=> ARG_2 = ARG_2_LO ∈ {0, 1}
                                                (begin (if-not-zero (- ARG_2_LO 1)
                                                                    ;; Thus ARG_2_LO != 1 <=> ARG_2_LO == 0
                                                                    (begin (vanishes! RES_HI)
                                                                           (= RES_LO 1)))
                                                       (if-not-zero ARG_2_LO
                                                                    ;; Thus ARG_2_LO != 0 <=> ARG_2_LO == 1
                                                                    (begin (= RES_HI ARG_1_HI)
                                                                           (= RES_LO ARG_1_LO))))
                                                ;; TINYE == 0 but OLI == 1 thus TINYB == 1
                                                (begin (= RES_HI ARG_1_HI)
                                                       (= RES_LO ARG_1_LO))))))))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;    1.8 aliases    ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun (A_3) ACC_A_3)
(defun (A_2) ACC_A_2)
(defun (A_1) ACC_A_1)
(defun (A_0) ACC_A_0)

;========
(defun (B_3) ACC_B_3)
(defun (B_2) ACC_B_2)
(defun (B_1) ACC_B_1)
(defun (B_0) ACC_B_0)

;========
(defun (C_3) ACC_C_3)
(defun (C_2) ACC_C_2)
(defun (C_1) ACC_C_1)
(defun (C_0) ACC_C_0)

;========
(defun (C'_3) (shift ACC_C_3 -8))
(defun (C'_2) (shift ACC_C_2 -8))
(defun (C'_1) (shift ACC_C_1 -8))
(defun (C'_0) (shift ACC_C_0 -8))

;========
(defun (H_3) ACC_H_3)
(defun (H_2) ACC_H_2)
(defun (H_1) ACC_H_1)
(defun (H_0) ACC_H_0)

;========
(defun (alpha)    (shift BITS -5))
(defun (beta_0)   (shift BITS -4))
(defun (beta_1)   (shift BITS -3))
(defun (eta)      (shift BITS -2))
(defun (mu_0)     (shift BITS -1))
(defun (mu_1)            BITS)

;========
(defun (beta) (+ (* 2 (beta_1)) (beta_0)))
(defun (mu)   (+ (* 2 (mu_1))   (mu_0)))    ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    1.9 nontrivial MUL regime    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint nontrivial-mul-regime ()
  (if-eq CT MMEDIUMMO
         ;; i.e. if INST == MUL
         (if-not-zero (- INST EVM_INST_EXP)
                      ;; byte decomposition constraints
                      (begin (= ARG_1_HI
                                (+ (* THETA (A_3)) (A_2)))
                             (= ARG_1_LO
                                (+ (* THETA (A_1)) (A_0)))
                             (= ARG_2_HI
                                (+ (* THETA (B_3)) (B_2)))
                             (= ARG_2_LO
                                (+ (* THETA (B_1)) (B_0)))
                             (= RES_HI
                                (+ (* THETA (C_3)) (C_2)))
                             (= RES_LO
                                (+ (* THETA (C_1)) (C_0)))
                             ;; multiplication per se
                             (set-multiplication (A_3)
                                                 (A_2)
                                                 (A_1)
                                                 (A_0)
                                                 (B_3)
                                                 (B_2)
                                                 (B_1)
                                                 (B_0)
                                                 (C_3)
                                                 (C_2)
                                                 (C_1)
                                                 (C_0)
                                                 (H_3)
                                                 (H_2)
                                                 (H_1)
                                                 (H_0)
                                                 (alpha)
                                                 (beta)
                                                 (eta)
                                                 (mu))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;    1.10 nontrivial EXP regime - zero result    ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (special-constraints-for-byte-c-0)
  (if-eq-else CT MMEDIUMMO
              (if-zero (A_0)
                       (vanishes! BYTE_C_0)
                       (= BYTE_C_0 1))
              (will-remain-constant! BYTE_C_0)))

(defun (preparations-for-a-lower-bound-on-the-2-adicity-of-the-base)
  (begin  ;; recall that BYTE_C_0 will be either 0 or 1 !
         ;; cf. (special-constraints-for-byte-c-0)
         (if-not-zero BYTE_C_0
                      (prepare-lower-bound-on-two-adicity BYTE_A_0
                                                          BYTE_C_1
                                                          BITS
                                                          BYTE_C_3
                                                          BYTE_H_3
                                                          BYTE_C_2
                                                          BYTE_H_2
                                                          CT))
         (if-not-zero (- 1 BYTE_C_0)
                      (prepare-lower-bound-on-two-adicity BYTE_A_1
                                                          BYTE_C_1
                                                          BITS
                                                          BYTE_C_3
                                                          BYTE_H_3
                                                          BYTE_C_2
                                                          BYTE_H_2
                                                          CT))))

(defun (nu2-byte-c-0-equals-1) (+ (* 8 BYTE_H_3) BYTE_H_2))
(defun (nu2-byte-c-0-equals-0) (+ (* 8 BYTE_H_3) BYTE_H_2 (* 8 MMEDIUM)))
;; θ · (B_3 + B_2 + B_1) + B_0
(defun (test-for-bytehood-of-arg-2) (+ (B_0)
                                       (* THETA (+ (B_3) (B_2) (B_1)))))

(defun (proving-the-vanishing-of-exp)
  (if-eq CT MMEDIUMMO
         (if-eq-else (test-for-bytehood-of-arg-2) BYTE_B_0
                     ;; ARG_2 is a byte
                     (begin (if-not-zero BYTE_C_0
                                         (= (* (B_0) (nu2-byte-c-0-equals-1)) (+ 256 (H_1))))
                            (if-not-zero (- 1 BYTE_C_0)
                                         (= (* (B_0) (nu2-byte-c-0-equals-0)) (+ 256 (H_1)))))
                     ;; ARG_2 isn't a byte
                     (begin (if-not-zero BYTE_C_0
                                         (= (nu2-byte-c-0-equals-1) (+ 1 (H_1))))
                            (debug (if-not-zero (- 1 BYTE_C_0)
                                                (= (nu2-byte-c-0-equals-0) (+ 1 (H_1)))))))))

(defconstraint prepare-lower-bound-on-the-2-adicity-of-the-base ()
  ;; sincde we will later impose RESV == 1 we will have
  ;; STAMP != 0 and thus INST ∈ {MUL, EXP}
  ;; INST != MUL <=> INST == EXP
  (if-not-zero (- INST EVM_INST_MUL)
               (if-zero OLI
                        (if-not-zero RESV
                                     ;; target constraints
                                     (begin (if-eq CT MMEDIUMMO
                                                   (begin (= ARG_1_HI
                                                             (+ (* THETA (A_3)) (A_2)))
                                                          (= ARG_1_LO
                                                             (+ (* THETA (A_1)) (A_0)))
                                                          ;;
                                                          (= ARG_2_HI
                                                             (+ (* THETA (B_3)) (B_2)))
                                                          (= ARG_2_LO
                                                             (+ (* THETA (B_1)) (B_0)))))
                                            (if-not-zero ARG_1_LO
                                                         (begin (special-constraints-for-byte-c-0)
                                                                (preparations-for-a-lower-bound-on-the-2-adicity-of-the-base)
                                                                (proving-the-vanishing-of-exp))))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;    1.11 nontrivial EXP regime - nonzero result    ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (target-arg1)
  (begin (= ARG_1_HI
            (+ (* THETA (A_3)) (A_2)))
         (= ARG_1_LO
            (+ (* THETA (A_1)) (A_0)))))

(defun (first-square-and-multiply)
  (if-not-zero (- (shift STAMP -8) STAMP)
               (begin (= ARG_1_HI
                         (+ (* THETA (C_3)) (C_2)))
                      (= ARG_1_LO
                         (+ (* THETA (C_1)) (C_0))))))

(defun (subsequent-square-and-multiply)
  (if-eq (shift STAMP -8) STAMP
         (if-zero SNM
                  ;; SQUARING
                  (set-multiplication (C'_3)
                                      (C'_2)
                                      (C'_1)
                                      (C'_0)
                                      (C'_3)
                                      (C'_2)
                                      (C'_1)
                                      (C'_0)
                                      (C_3)
                                      (C_2)
                                      (C_1)
                                      (C_0)
                                      (H_3)
                                      (H_2)
                                      (H_1)
                                      (H_0)
                                      (alpha)
                                      (beta)
                                      (eta)
                                      (mu))
                  ;; MULTIPLY
                  (set-multiplication (C'_3)
                                      (C'_2)
                                      (C'_1)
                                      (C'_0)
                                      (A_3)
                                      (A_2)
                                      (A_1)
                                      (A_0)
                                      (C_3)
                                      (C_2)
                                      (C_1)
                                      (C_0)
                                      (H_3)
                                      (H_2)
                                      (H_1)
                                      (H_0)
                                      (alpha)
                                      (beta)
                                      (eta)
                                      (mu)))))

(defun (final-square-and-multiply)
  (if-not-zero (will-remain-constant! STAMP)
               (begin (= RES_HI
                         (+ (* THETA (C_3)) (C_2)))
                      (= RES_LO
                         (+ (* THETA (C_1)) (C_0))))))

(defconstraint nontrivial-exp-regime-nonzero-result ()
  (if-eq INST EVM_INST_EXP
         (if-zero RESV
                  (if-eq CT MMEDIUMMO
                         (begin (target-arg1)
                                (first-square-and-multiply)
                                (subsequent-square-and-multiply)
                                (final-square-and-multiply))))))
