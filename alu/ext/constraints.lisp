(module ext)

(defconst
  THETA  18446744073709551616                     ;18446744073709551616 = 256^8
  THETA2 340282366920938463463374607431768211456) ;340282366920938463463374607431768211456 = 256^16

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    6.3 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint heartbeat ()
  (begin (or! (will-remain-constant! STAMP) (will-inc! STAMP 1))
         (if-zero STAMP
                  (begin (vanishes! CT)
                         (vanishes! OLI)
                         (vanishes! INST)))
         (if-not-zero (will-remain-constant! STAMP)
                      (vanishes! (next CT)))
         (if-not-zero STAMP
                      (begin (if-not-zero OLI
                                          (will-inc! STAMP 1)
                                          (if-eq-else CT MMEDIUMMO
                                                      (will-inc! STAMP 1)
                                                      (begin (will-inc! CT 1)
                                                             (vanishes! (next OLI)))))
                             (or! (eq! INST EVM_INST_MULMOD) (eq! INST EVM_INST_ADDMOD))))))

(defconstraint last-row (:domain {-1} :guard STAMP)
  (if-zero OLI
           (eq! CT MMEDIUMMO)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    6.4 stamp constancies    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP ARG_1_HI)
         (stamp-constancy STAMP ARG_1_LO)
         (stamp-constancy STAMP ARG_2_HI)
         (stamp-constancy STAMP ARG_2_LO)
         (stamp-constancy STAMP ARG_3_HI)
         (stamp-constancy STAMP ARG_3_LO)
         (stamp-constancy STAMP RES_HI)
         (stamp-constancy STAMP RES_LO)
         (stamp-constancy STAMP INST)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    6.6 Byte decomposition constraints    ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint byte-decompositions ()
  (begin (byte-decomposition CT ACC_A_3 BYTE_A_3)
         (byte-decomposition CT ACC_A_2 BYTE_A_2)
         (byte-decomposition CT ACC_A_1 BYTE_A_1)
         (byte-decomposition CT ACC_A_0 BYTE_A_0)
         ;;
         (byte-decomposition CT ACC_B_3 BYTE_B_3)
         (byte-decomposition CT ACC_B_2 BYTE_B_2)
         (byte-decomposition CT ACC_B_1 BYTE_B_1)
         (byte-decomposition CT ACC_B_0 BYTE_B_0)
         ;;
         (byte-decomposition CT ACC_C_3 BYTE_C_3)
         (byte-decomposition CT ACC_C_2 BYTE_C_2)
         (byte-decomposition CT ACC_C_1 BYTE_C_1)
         (byte-decomposition CT ACC_C_0 BYTE_C_0)
         ;;
         (byte-decomposition CT ACC_DELTA_3 BYTE_DELTA_3)
         (byte-decomposition CT ACC_DELTA_2 BYTE_DELTA_2)
         (byte-decomposition CT ACC_DELTA_1 BYTE_DELTA_1)
         (byte-decomposition CT ACC_DELTA_0 BYTE_DELTA_0)
         ;;
         (byte-decomposition CT ACC_Q_7 BYTE_Q_7)
         (byte-decomposition CT ACC_Q_6 BYTE_Q_6)
         (byte-decomposition CT ACC_Q_5 BYTE_Q_5)
         (byte-decomposition CT ACC_Q_4 BYTE_Q_4)
         (byte-decomposition CT ACC_Q_3 BYTE_Q_3)
         (byte-decomposition CT ACC_Q_2 BYTE_Q_2)
         (byte-decomposition CT ACC_Q_1 BYTE_Q_1)
         (byte-decomposition CT ACC_Q_0 BYTE_Q_0)
         ;;
         (byte-decomposition CT ACC_R_3 BYTE_R_3)
         (byte-decomposition CT ACC_R_2 BYTE_R_2)
         (byte-decomposition CT ACC_R_1 BYTE_R_1)
         (byte-decomposition CT ACC_R_0 BYTE_R_0)
         ;;
         (byte-decomposition CT ACC_H_5 BYTE_H_5)
         (byte-decomposition CT ACC_H_4 BYTE_H_4)
         (byte-decomposition CT ACC_H_3 BYTE_H_3)
         (byte-decomposition CT ACC_H_2 BYTE_H_2)
         (byte-decomposition CT ACC_H_1 BYTE_H_1)
         (byte-decomposition CT ACC_H_0 BYTE_H_0)
         ;;
         (byte-decomposition CT ACC_I_6 BYTE_I_6)
         (byte-decomposition CT ACC_I_5 BYTE_I_5)
         (byte-decomposition CT ACC_I_4 BYTE_I_4)
         (byte-decomposition CT ACC_I_3 BYTE_I_3)
         (byte-decomposition CT ACC_I_2 BYTE_I_2)
         (byte-decomposition CT ACC_I_1 BYTE_I_1)
         (byte-decomposition CT ACC_I_0 BYTE_I_0)
         ;;
         (byte-decomposition CT ACC_J_7 BYTE_J_7)
         (byte-decomposition CT ACC_J_6 BYTE_J_6)
         (byte-decomposition CT ACC_J_5 BYTE_J_5)
         (byte-decomposition CT ACC_J_4 BYTE_J_4)
         (byte-decomposition CT ACC_J_3 BYTE_J_3)
         (byte-decomposition CT ACC_J_2 BYTE_J_2)
         (byte-decomposition CT ACC_J_1 BYTE_J_1)
         (byte-decomposition CT ACC_J_0 BYTE_J_0)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    6.7 OLI constraints    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint bit-1-constraints ()
  (if-not-zero STAMP
               (begin (if-not-zero (- INST EVM_INST_MULMOD) (vanishes! BIT_1))
                      (if-not-zero ARG_1_HI                 (vanishes! BIT_1))
                      (if-zero ARG_1_HI
                               (if-not-zero (- INST EVM_INST_ADDMOD)
                                            (if-zero ARG_1_LO
                                                     (eq! BIT_1 1)
                                                     (vanishes! BIT_1)))))))

(defconstraint bit-2-constraints ()
  (if-not-zero STAMP
               (begin (if-not-zero (- INST EVM_INST_MULMOD)  (vanishes! BIT_2))
                      (if-not-zero ARG_2_HI                  (vanishes! BIT_2))
                      (if-zero ARG_2_HI
                               (if-eq INST EVM_INST_MULMOD
                                      (if-zero ARG_2_LO
                                               (eq! BIT_2 1)
                                               (vanishes! BIT_2)))))))

(defconstraint bit-3-constraints ()
               (if-not-zero STAMP
                            (begin
                              (if-not-zero   ARG_3_HI
                                             ;; ARG_3_HI ≠ 0
                                             (vanishes! BIT_3)
                                             ;; ARG_3_HI = 0
                                             (if-not-zero   (*   ARG_3_LO   (-   1   ARG_3_LO))
                                                            (eq!    BIT_3   0)
                                                            (eq!    BIT_3   1))))))

(defconstraint oli-constraints ()
  (if-not-zero STAMP
               (eq! OLI
                  (- (+ BIT_1 BIT_2 BIT_3 (* BIT_1 BIT_2 BIT_3))
                     (+ (* BIT_1 BIT_2) (* BIT_2 BIT_3) (* BIT_3 BIT_1))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    6.8 trivial constraints    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint oli-implies-vanishing ()
  (if-not-zero OLI
               (begin (vanishes! RES_HI)
                      (vanishes! RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;    6.9 aliases    ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;
(defun (A_3)
  ACC_A_3)

(defun (A_2)
  ACC_A_2)

(defun (A_1)
  ACC_A_1)

(defun (A_0)
  ACC_A_0)

;;
(defun (B_3)
  ACC_B_3)

(defun (B_2)
  ACC_B_2)

(defun (B_1)
  ACC_B_1)

(defun (B_0)
  ACC_B_0)

;;
(defun (C_3)
  ACC_C_3)

(defun (C_2)
  ACC_C_2)

(defun (C_1)
  ACC_C_1)

(defun (C_0)
  ACC_C_0)

;;
(defun (R_3)
  ACC_R_3)

(defun (R_2)
  ACC_R_2)

(defun (R_1)
  ACC_R_1)

(defun (R_0)
  ACC_R_0)

;;
(defun (Delta_3)
  ACC_DELTA_3)

(defun (Delta_2)
  ACC_DELTA_2)

(defun (Delta_1)
  ACC_DELTA_1)

(defun (Delta_0)
  ACC_DELTA_0)

;;
(defun (Q_7)
  ACC_Q_7)

(defun (Q_6)
  ACC_Q_6)

(defun (Q_5)
  ACC_Q_5)

(defun (Q_4)
  ACC_Q_4)

(defun (Q_3)
  ACC_Q_3)

(defun (Q_2)
  ACC_Q_2)

(defun (Q_1)
  ACC_Q_1)

(defun (Q_0)
  ACC_Q_0)

;;
(defun (H_5)
  ACC_H_5)

(defun (H_4)
  ACC_H_4)

(defun (H_3)
  ACC_H_3)

(defun (H_2)
  ACC_H_2)

(defun (H_1)
  ACC_H_1)

(defun (H_0)
  ACC_H_0)

;;
(defun (I_6)
  ACC_I_6)

(defun (I_5)
  ACC_I_5)

(defun (I_4)
  ACC_I_4)

(defun (I_3)
  ACC_I_3)

(defun (I_2)
  ACC_I_2)

(defun (I_1)
  ACC_I_1)

(defun (I_0)
  ACC_I_0)

;;
(defun (J_7)
  ACC_J_7)

(defun (J_6)
  ACC_J_6)

(defun (J_5)
  ACC_J_5)

(defun (J_4)
  ACC_J_4)

(defun (J_3)
  ACC_J_3)

(defun (J_2)
  ACC_J_2)

(defun (J_1)
  ACC_J_1)

(defun (J_0)
  ACC_J_0)

;;
(defun (lt_0)
  (shift CMP -7))

(defun (lt_1)
  (shift CMP -6))

(defun (lt_2)
  (shift CMP -5))

(defun (lt_3)
  (shift CMP -4))

(defun (eq_0)
  (shift CMP -3))

(defun (eq_1)
  (shift CMP -2))

(defun (eq_2)
  (shift CMP -1))

(defun (eq_3)
  CMP)

;;
(defun (alpha)
  (shift OF_H -7))

(defun (beta_0)
  (shift OF_H -6))

(defun (beta_1)
  (shift OF_H -5))

(defun (gamma)
  (shift OF_H -4))

(defun (beta)
  (+ (* 2 (beta_1)) (beta_0)))

;;
(defun (sigma)
  (shift OF_I -7))

(defun (tau_0)
  (shift OF_I -6))

(defun (tau_1)
  (shift OF_I -5))

(defun (rho_0)
  (shift OF_I -4))

(defun (rho_1)
  (shift OF_I -3))

(defun (tau)
  (+ (* 2 (tau_1)) (tau_0)))

(defun (rho)
  (+ (* 2 (rho_1)) (rho_0)))

;;
(defun (phi_0)
  (shift OF_J -7))

(defun (phi_1)
  (shift OF_J -6))

(defun (psi_0)
  (shift OF_J -5))

(defun (psi_1)
  (shift OF_J -4))

(defun (psi_2)
  (shift OF_J -3))

(defun (chi_0)
  (shift OF_J -2))

(defun (chi_1)
  (shift OF_J -1))

(defun (chi_2)
  OF_J)

(defun (phi)
  (+ (* 2 (phi_1)) (phi_0)))

(defun (psi)
  (+ (* 4 (psi_2)) (* 2 (psi_1)) (psi_0)))

(defun (chi)
  (+ (* 4 (chi_2)) (* 2 (chi_1)) (chi_0)))

;;
(defun (lambda)
  (shift OF_RES -7))

(defun (mu_0)
  (shift OF_RES -6))

(defun (mu_1)
  (shift OF_RES -5))

(defun (nu_0)
  (shift OF_RES -4))

(defun (nu_1)
  (shift OF_RES -3))

(defun (mu)
  (+ (* 2 (mu_1)) (mu_0)))

(defun (nu)
  (+ (* 2 (nu_1)) (nu_0)))

;;
(defun (A_HI)
  (+ (* THETA (A_3)) (A_2)))

(defun (A_LO)
  (+ (* THETA (A_1)) (A_0)))

(defun (B_HI)
  (+ (* THETA (B_3)) (B_2)))

(defun (B_LO)
  (+ (* THETA (B_1)) (B_0)))

(defun (C_HI)
  (+ (* THETA (C_3)) (C_2)))

(defun (C_LO)
  (+ (* THETA (C_1)) (C_0)))

(defun (R_HI)
  (+ (* THETA (R_3)) (R_2)))

(defun (R_LO)
  (+ (* THETA (R_1)) (R_0)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    6.10.2 target constraints    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target-constraints ()
  (if-eq CT MMEDIUMMO
         (begin (eq! (A_HI) ARG_1_HI)
                (eq! (A_LO) ARG_1_LO)
                (eq! (B_HI) ARG_2_HI)
                (eq! (B_LO) ARG_2_LO)
                (eq! (C_HI) ARG_3_HI)
                (eq! (C_LO) ARG_3_LO)
                (eq! (R_HI) RES_HI)
                (eq! (R_LO) RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    6.10.3 comparison constraints    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint comparisons ()
  (if-eq CT MMEDIUMMO
         (begin (eq! (* (- (* 2 (lt_3)) 1)
                      (- (C_3) (R_3)))
                   (+ (Delta_3) (lt_3)))
                (eq! (* (- (* 2 (lt_2)) 1)
                      (- (C_2) (R_2)))
                   (+ (Delta_2) (lt_2)))
                (eq! (* (- (* 2 (lt_1)) 1)
                      (- (C_1) (R_1)))
                   (+ (Delta_1) (lt_1)))
                (eq! (* (- (* 2 (lt_0)) 1)
                      (- (C_0) (R_0)))
                   (+ (Delta_0) (lt_0)))
                (if-eq-else (C_3) (R_3) (eq! (eq_3) 1) (vanishes! (eq_3)))
                (if-eq-else (C_2) (R_2) (eq! (eq_2) 1) (vanishes! (eq_2)))
                (if-eq-else (C_1) (R_1) (eq! (eq_1) 1) (vanishes! (eq_1)))
                (if-eq-else (C_0) (R_0) (eq! (eq_0) 1) (vanishes! (eq_0))))))

(defconstraint order ()
  (if-eq CT MMEDIUMMO
         (eq! 1
            (+ (lt_3)
               (* (eq_3) (lt_2))
               (* (eq_3) (eq_2) (lt_1))
               (* (eq_3) (eq_2) (eq_1) (lt_0))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    6.10.4 general constraints    ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint auxiliary-products-of-arguments ()
  (if-eq CT MMEDIUMMO
         (begin (eq! (+ (* (A_1) (B_0)) (* (A_0) (B_1)))
                   (+ (* THETA2 (alpha)) (* THETA (H_1)) (H_0)))
                (eq! (+ (* (A_3) (B_0)) (* (A_2) (B_1)) (* (A_1) (B_2)) (* (A_0) (B_3)))
                   (+ (* THETA2 (beta)) (* THETA (H_3)) (H_2)))
                (eq! (+ (* (A_3) (B_2)) (* (A_2) (B_3)))
                   (+ (* THETA2 (gamma)) (* THETA (H_5)) (H_4))))))

(defconstraint auxiliary-computations-for-euclidean-division ()
  (if-eq CT MMEDIUMMO
         (begin (eq! (+ (* (Q_1) (C_0)) (* (Q_0) (C_1)))
                   (+ (* THETA2 (sigma)) (* THETA (I_1)) (I_0)))
                (eq! (+ (* (Q_3) (C_0)) (* (Q_2) (C_1)) (* (Q_1) (C_2)) (* (Q_0) (C_3)))
                   (+ (* THETA2 (tau)) (* THETA (I_3)) (I_2)))
                (eq! (+ (* (Q_5) (C_0)) (* (Q_4) (C_1)) (* (Q_3) (C_2)) (* (Q_2) (C_3)))
                   (+ (* THETA2 (rho)) (* THETA (I_5)) (I_4)))
                (eq! (+ (* (Q_7) (C_0)) (* (Q_6) (C_1)) (* (Q_5) (C_2)) (* (Q_4) (C_3)))
                   (I_6)))))

(defconstraint vanishing-of-very-high-parts ()
  (if-eq CT MMEDIUMMO
         (vanishes! (+ (* (Q_7) (C_1))
                       (* (Q_6) (C_2))
                       (* (Q_5) (C_3))
                       (* (Q_7) (C_2))
                       (* (Q_6) (C_3))
                       (* (Q_7) (C_3))))))

(defconstraint euclidean-division-per-se ()
  (if-eq CT MMEDIUMMO
         (begin (eq! (+ (* (Q_0) (C_0)) (* THETA (I_0)) (* THETA (R_1)) (R_0))
                   (+ (* THETA2 (phi)) (* THETA (J_1)) (J_0)))
                ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                (eq! (+ (phi)
                      (I_1)
                      (* THETA (sigma))
                      (* (Q_2) (C_0))
                      (* (Q_1) (C_1))
                      (* (Q_0) (C_2))
                      (* THETA (I_2))
                      (* THETA (R_3))
                      (R_2))
                   (+ (* THETA2 (psi)) (* THETA (J_3)) (J_2)))
                ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                (eq! (+ (psi)
                      (I_3)
                      (* THETA (tau))
                      (* (Q_4) (C_0))
                      (* (Q_3) (C_1))
                      (* (Q_2) (C_2))
                      (* (Q_1) (C_3))
                      (* THETA (I_4)))
                   (+ (* THETA2 (chi)) (* THETA (J_5)) (J_4)))
                ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                (eq! (+ (chi)
                      (I_5)
                      (* THETA (rho))
                      (* (Q_6) (C_0))
                      (* (Q_5) (C_1))
                      (* (Q_4) (C_2))
                      (* (Q_3) (C_3))
                      (* THETA (I_6)))
                   (+ (* THETA (J_7)) (J_6))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    6.10.6 constraints for ADDMOD    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint addmod-constraints ()
  (if-eq CT MMEDIUMMO
         (if-not-zero (- INST EVM_INST_MULMOD)
                      (begin (eq! (+ ARG_1_LO ARG_2_LO)
                                (+ (* THETA2 (lambda)) (* THETA (J_1)) (J_0)))
                             ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                             (eq! (+ (lambda) ARG_1_HI ARG_2_HI)
                                (+ (* THETA2 (mu)) (* THETA (J_3)) (J_2)))
                             (eq! (mu) (J_4))
                             (vanishes! (+ (J_5) (J_6) (J_7)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    6.10.6 constraints for MULMOD    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint mulmod-constraints ()
  (if-eq CT MMEDIUMMO
         (if-not-zero (- INST EVM_INST_ADDMOD)
                      (begin (eq! (+ (* (A_0) (B_0)) (* THETA (H_0)))
                                (+ (* THETA2 (lambda)) (* THETA (J_1)) (J_0)))
                             ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                             (eq! (+ (lambda)
                                   (H_1)
                                   (* THETA (alpha))
                                   (* (A_2) (B_0))
                                   (* (A_1) (B_1))
                                   (* (A_0) (B_2))
                                   (* THETA (H_2)))
                                (+ (* THETA2 (mu)) (* THETA (J_3)) (J_2)))
                             ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                             (eq! (+ (mu)
                                   (H_3)
                                   (* THETA (beta))
                                   (* (A_3) (B_1))
                                   (* (A_2) (B_2))
                                   (* (A_1) (B_3))
                                   (* THETA (H_4)))
                                (+ (* THETA2 (nu)) (* THETA (J_5)) (J_4)))
                             ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                             (eq! (+ (nu) (H_5) (* THETA (gamma)) (* (A_3) (B_3)))
                                (+ (* THETA (J_7)) (J_6)))))))


