(module mod)

(defconst 
  THETA                  18446744073709551616                     ;18446744073709551616 = 256^8
  THETA2                 340282366920938463463374607431768211456  ;340282366920938463463374607431768211456 = 256^16
  THETA_SQUARED_OVER_TWO 170141183460469231731687303715884105728) ;170141183460469231731687303715884105728 = (1/2)*THETA2

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                              ;;;;
;;;;    _xxX=Aliases69420=Xxx_    ;;;;
;;;;                              ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ARG1_3)
  ACC_1_3)

(defun (ARG1_2)
  ACC_1_2)

(defun (ARG2_3)
  ACC_2_3)

(defun (ARG2_2)
  ACC_2_2)

(defun (B_3)
  ACC_B_3)

(defun (B_2)
  ACC_B_2)

(defun (B_1)
  ACC_B_1)

(defun (B_0)
  ACC_B_0)

(defun (Q_3)
  ACC_Q_3)

(defun (Q_2)
  ACC_Q_2)

(defun (Q_1)
  ACC_Q_1)

(defun (Q_0)
  ACC_Q_0)

(defun (R_3)
  ACC_R_3)

(defun (R_2)
  ACC_R_2)

(defun (R_1)
  ACC_R_1)

(defun (R_0)
  ACC_R_0)

(defun (Delta_3)
  ACC_DELTA_3)

(defun (Delta_2)
  ACC_DELTA_2)

(defun (Delta_1)
  ACC_DELTA_1)

(defun (Delta_0)
  ACC_DELTA_0)

(defun (H_2)
  ACC_H_2)

(defun (H_1)
  ACC_H_1)

(defun (H_0)
  ACC_H_0)

(defun (sgn_1)
  (shift MSB_1 -7))

(defun (sgn_2)
  (shift MSB_2 -7))

;aliases for the comparison columns
(defun (lt_0)
  (shift CMP_1 -7))

(defun (eq_0)
  (shift CMP_2 -7))

(defun (lt_1)
  (shift CMP_1 -6))

(defun (eq_1)
  (shift CMP_2 -6))

(defun (lt_2)
  (shift CMP_1 -5))

(defun (eq_2)
  (shift CMP_2 -5))

(defun (lt_3)
  (shift CMP_1 -4))

(defun (eq_3)
  (shift CMP_2 -4))

(defun (alpha)
  (shift CMP_2 -3))

(defun (beta_0)
  (shift CMP_2 -2))

(defun (beta_1)
  (shift CMP_2 -1))

(defun (beta)
  (+ (* 2 (beta_1)) (beta_0)))

(defun (R_HI)
  (+ (* THETA (R_3)) (R_2)))

(defun (R_LO)
  (+ (* THETA (R_1)) (R_0)))

(defun (Q_HI)
  (+ (* THETA (Q_3)) (Q_2)))

(defun (Q_LO)
  (+ (* THETA (Q_1)) (Q_0)))

(defun (B_LO)
  (+ (* THETA (B_1)) (B_0)))

(defun (B_HI)
  (+ (* THETA (B_3)) (B_2)))

(defun (A_LO)
  (- (+ (* (B_0) (Q_0)) (* THETA (H_0)) (R_LO))
     (* THETA2 (beta))))

(defun (A_HI)
  (+ (beta)
     (H_1)
     (* THETA (alpha))
     (* (B_0) (Q_2))
     (* (B_1) (Q_1))
     (* (B_2) (Q_0))
     (* THETA (H_2))
     (R_HI)))

;; bit decompositions of the most significant bytes
(defun (bit-dec-msb1)
  (+ (* 128 (shift MSB_1 -7))
     (* 64 (shift MSB_1 -6))
     (* 32 (shift MSB_1 -5))
     (* 16 (shift MSB_1 -4))
     (* 8 (shift MSB_1 -3))
     (* 4 (shift MSB_1 -2))
     (* 2 (shift MSB_1 -1))
     MSB_1))

(defun (bit-dec-msb2)
  (+ (* 128 (shift MSB_2 -7))
     (* 64 (shift MSB_2 -6))
     (* 32 (shift MSB_2 -5))
     (* 16 (shift MSB_2 -4))
     (* 8 (shift MSB_2 -3))
     (* 4 (shift MSB_2 -2))
     (* 2 (shift MSB_2 -1))
     MSB_2))

(defun (set-negative zHi zLo yHi yLo)
  (if-not-zero yLo
               (begin (= zHi (- THETA2 yHi 1))
                      (= zLo (- THETA2 yLo)))
               (begin (vanishes! zLo)
                      (if-zero (* yHi (- THETA_SQUARED_OVER_TWO yHi))
                               (= zHi yHi)
                               (= zHi (- THETA2 yHi))))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    4.5 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint heartbeat ()
  (begin (* (will-remain-constant! STAMP) (will-inc! STAMP 1))
         (if-zero STAMP
                  (begin (vanishes! CT)
                         (vanishes! OLI)))
         (if-not-zero (will-remain-constant! STAMP)
                      (vanishes! (next CT)))
         (if-not-zero STAMP
                      (if-not-zero OLI
                                   (will-inc! STAMP 1)
                                   (if-eq-else CT MMEDIUMMO
                                               (will-inc! STAMP 1)
                                               (begin (will-inc! CT 1)
                                                      (vanishes! (next OLI))))))))

(defconstraint last-row (:domain {-1})
  (if-not-zero STAMP
               (if-zero OLI
                        (= CT MMEDIUMMO))))

(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP ARG_1_HI)
         (stamp-constancy STAMP ARG_1_LO)
         (stamp-constancy STAMP ARG_2_HI)
         (stamp-constancy STAMP ARG_2_LO)
         (stamp-constancy STAMP RES_HI)
         (stamp-constancy STAMP RES_LO)
         (stamp-constancy STAMP INST)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                               ;;
;;    4.6 Binary, bytehood and byte decomposition constraints    ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint binary-columns ()
  (begin (is-binary OLI)
         (is-binary CMP_1)
         (is-binary CMP_2)
         (is-binary MSB_1)
         (is-binary MSB_2)))

;TODO bytehood constraints!
(defconstraint byte-decompositions ()
  (begin (byte-decomposition CT ACC_1_3 BYTE_1_3)
         (byte-decomposition CT ACC_1_2 BYTE_1_2)
         (byte-decomposition CT ACC_2_3 BYTE_2_3)
         (byte-decomposition CT ACC_2_2 BYTE_2_2)
         (byte-decomposition CT ACC_B_3 BYTE_B_3)
         (byte-decomposition CT ACC_B_2 BYTE_B_2)
         (byte-decomposition CT ACC_B_1 BYTE_B_1)
         (byte-decomposition CT ACC_B_0 BYTE_B_0)
         (byte-decomposition CT ACC_Q_3 BYTE_Q_3)
         (byte-decomposition CT ACC_Q_2 BYTE_Q_2)
         (byte-decomposition CT ACC_Q_1 BYTE_Q_1)
         (byte-decomposition CT ACC_Q_0 BYTE_Q_0)
         (byte-decomposition CT ACC_R_3 BYTE_R_3)
         (byte-decomposition CT ACC_R_2 BYTE_R_2)
         (byte-decomposition CT ACC_R_1 BYTE_R_1)
         (byte-decomposition CT ACC_R_0 BYTE_R_0)
         (byte-decomposition CT ACC_DELTA_3 BYTE_DELTA_3)
         (byte-decomposition CT ACC_DELTA_2 BYTE_DELTA_2)
         (byte-decomposition CT ACC_DELTA_1 BYTE_DELTA_1)
         (byte-decomposition CT ACC_DELTA_0 BYTE_DELTA_0)
         (byte-decomposition CT ACC_H_2 BYTE_H_2)
         (byte-decomposition CT ACC_H_1 BYTE_H_1)
         (byte-decomposition CT ACC_H_0 BYTE_H_0)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    4.7 OLI constraints    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint oli-constraints (:guard STAMP)
  (if-zero ARG_2_HI
           (if-zero ARG_2_LO
                    (begin (= OLI 1)
                           (vanishes! RES_HI)
                           (vanishes! RES_LO))
                    (vanishes! OLI))
           (vanishes! OLI)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    4.7 target constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target-constraints (:guard STAMP)
  (if-zero OLI
           (if-eq CT MMEDIUMMO
                  (begin (= ARG_1_HI
                            (+ (* THETA (ARG1_3)) (ARG1_2)))
                         (= ARG_2_HI
                            (+ (* THETA (ARG2_3)) (ARG2_2)))
                         ;
                         (= (shift BYTE_1_3 -7) (bit-dec-msb1))
                         (= (shift BYTE_2_3 -7) (bit-dec-msb2))
                         ;
                         (= (+ (Delta_3) (lt_3))
                            (* (- (* 2 (lt_3)) 1)
                               (- (B_3) (R_3))))
                         (= (+ (Delta_2) (lt_2))
                            (* (- (* 2 (lt_2)) 1)
                               (- (B_2) (R_2))))
                         (= (+ (Delta_1) (lt_1))
                            (* (- (* 2 (lt_1)) 1)
                               (- (B_1) (R_1))))
                         (= (+ (Delta_0) (lt_0))
                            (* (- (* 2 (lt_0)) 1)
                               (- (B_0) (R_0))))
                         ;
                         (= (+ (* (B_0) (Q_1)) (* (B_1) (Q_0)))
                            (+ (* THETA2 (alpha)) (* THETA (H_1)) (H_0)))
                         (= (+ (* (B_0) (Q_3)) (* (B_1) (Q_2)) (* (B_2) (Q_1)) (* (B_3) (Q_0)))
                            (H_2))
                         (vanishes! (+ (* (B_1) (Q_3))
                                       (* (B_2) (Q_2))
                                       (* (B_3) (Q_1))
                                       (* (B_2) (Q_3))
                                       (* (B_3) (Q_2))
                                       (* (B_3) (Q_3))))
                         (if-zero (* DEC_SIGNED (sgn_1))
                                  (begin (= (A_HI) ARG_1_HI)
                                         (= (A_LO) ARG_1_LO))
                                  (if-zero ARG_1_LO
                                           (begin (= (A_HI) (- THETA2 ARG_1_HI))
                                                  (vanishes! (A_LO)))
                                           (begin (= (A_HI) (- THETA2 ARG_1_HI 1))
                                                  (= (A_LO) (- THETA2 ARG_1_LO)))))
                         ;; 5.9.5-1 compressed in a single expression
                         (if-zero (* DEC_SIGNED (sgn_2))
                                  (begin (= (B_HI) ARG_2_HI)
                                         (= (B_LO) ARG_2_LO))
                                  (if-zero ARG_2_LO
                                           (begin (= (B_HI) (- THETA2 ARG_2_HI))
                                                  (vanishes! (B_LO)))
                                           (begin (= (B_HI) (- THETA2 ARG_2_HI 1))
                                                  (= (B_LO) (- THETA2 ARG_2_LO)))))
                         (if-eq-else (B_3) (R_3) (= (eq_3) 1) (vanishes! (eq_3)))
                         (if-eq-else (B_2) (R_2) (= (eq_2) 1) (vanishes! (eq_2)))
                         (if-eq-else (B_1) (R_1) (= (eq_1) 1) (vanishes! (eq_1)))
                         (if-eq-else (B_0) (R_0) (= (eq_0) 1) (vanishes! (eq_0)))
                         (= 1
                            (+ (lt_3)
                               (* (eq_3) (lt_2))
                               (* (eq_3) (eq_2) (lt_1))
                               (* (eq_3) (eq_2) (eq_1) (lt_0))))
                         (if-zero DEC_SIGNED
                                  ;; ♦SIGNED = 0
                                  (if-zero DEC_OUTPUT
                                           ;; ♦OUTPUT = 0
                                           (begin (= RES_HI (R_HI))
                                                  (= RES_LO (R_LO)))
                                           ;; ♦OUTPUT = 1
                                           (begin (= RES_HI (Q_HI))
                                                  (= RES_LO (Q_LO))))
                                  ;; ♦SIGNED = 1
                                  (if-zero DEC_OUTPUT
                                           ;; ♦OUTPUT = 0
                                           (if-zero (sgn_1)
                                                    ;; sgn_1 = 0
                                                    (begin (= RES_HI (R_HI))
                                                           (= RES_LO (R_LO)))
                                                    ;; sgn_1 = 1
                                                    (set-negative RES_HI RES_LO (R_HI) (R_LO)))
                                           ;; (if-zero (R_1)
                                           ;;     (if-zero (R_0)
                                           ;;         (begin
                                           ;;             (= RES_HI (- THETA2 (R_HI)))
                                           ;;             (vanishes! RES_LO))
                                           ;;         (begin
                                           ;;             (= RES_HI (- THETA2 (R_HI) 1))
                                           ;;             (= RES_LO (- THETA2 (R_LO)))))
                                           ;;     (begin
                                           ;;         (= RES_HI (- THETA2 (R_HI) 1))
                                           ;;         (= RES_LO (- THETA2 (R_LO))))))
                                           ;; ♦OUTPUT = 1
                                           (if-eq-else (sgn_1) (sgn_2)
                                                       ;; sgn_1 == sgn_2
                                                       (begin (= RES_HI (Q_HI))
                                                              (= RES_LO (Q_LO)))
                                                       ;; sgn_1 != sgn_2
                                                       (set-negative RES_HI RES_LO (Q_HI) (Q_LO)))))))))


