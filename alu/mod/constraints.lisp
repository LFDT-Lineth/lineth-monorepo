(module mod)

;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;
;;;;               ;;;;
;;;;    Aliases    ;;;;
;;;;               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;

(defun    (ARG1_3)     ACC_1_3)
(defun    (ARG1_2)     ACC_1_2)
(defun    (ARG2_3)     ACC_2_3)
(defun    (ARG2_2)     ACC_2_2)
(defun    (B_3)        ACC_B_3)
(defun    (B_2)        ACC_B_2)
(defun    (B_1)        ACC_B_1)
(defun    (B_0)        ACC_B_0)
(defun    (Q_3)        ACC_Q_3)
(defun    (Q_2)        ACC_Q_2)
(defun    (Q_1)        ACC_Q_1)
(defun    (Q_0)        ACC_Q_0)
(defun    (R_3)        ACC_R_3)
(defun    (R_2)        ACC_R_2)
(defun    (R_1)        ACC_R_1)
(defun    (R_0)        ACC_R_0)
(defun    (Delta_3)    ACC_DELTA_3)
(defun    (Delta_2)    ACC_DELTA_2)
(defun    (Delta_1)    ACC_DELTA_1)
(defun    (Delta_0)    ACC_DELTA_0)
(defun    (H_2)        ACC_H_2)
(defun    (H_1)        ACC_H_1)
(defun    (H_0)        ACC_H_0) ;; ""
(defun    (sgn_1)      (shift MSB_1 -7))
(defun    (sgn_2)      (shift MSB_2 -7))
(defun    (lt_0)       (shift CMP_1 -7))
(defun    (eq_0)       (shift CMP_2 -7))
(defun    (lt_1)       (shift CMP_1 -6))
(defun    (eq_1)       (shift CMP_2 -6))
(defun    (lt_2)       (shift CMP_1 -5))
(defun    (eq_2)       (shift CMP_2 -5))
(defun    (lt_3)       (shift CMP_1 -4))
(defun    (eq_3)       (shift CMP_2 -4))
(defun    (alpha)      (shift CMP_2 -3))
(defun    (beta_0)     (shift CMP_2 -2))
(defun    (beta_1)     (shift CMP_2 -1))
(defun    (beta)       (+ (* 2 (beta_1)) (beta_0)))
(defun    (R_HI)       (+ (* THETA (R_3)) (R_2)))
(defun    (R_LO)       (+ (* THETA (R_1)) (R_0)))
(defun    (Q_HI)       (+ (* THETA (Q_3)) (Q_2)))
(defun    (Q_LO)       (+ (* THETA (Q_1)) (Q_0)))

;; absolute value shorthands
(defun    (ABS_2_HI)       (+ (* THETA (B_3)) (B_2)))
(defun    (ABS_2_LO)       (+ (* THETA (B_1)) (B_0)))
(defun    (ABS_1_HI)   (+ (beta)
                          (H_1)
                          (* THETA (alpha))
                          (* (B_0) (Q_2))
                          (* (B_1) (Q_1))
                          (* (B_2) (Q_0))
                          (* THETA (H_2))
                          (R_HI)))
(defun    (ABS_1_LO)   (- (+ (* (B_0) (Q_0))
                             (* THETA (H_0))
                             (R_LO))
                          (* THETA2 (beta))))

;; alisases for decoding inst
(defun   (flag_sum)      (force-bin (+ IS_SMOD IS_MOD IS_SDIV IS_DIV)))
(defun   (weight_sum)    (+ (* EVM_INST_SMOD IS_SMOD) (* EVM_INST_MOD IS_MOD) (* EVM_INST_SDIV IS_SDIV) (* EVM_INST_DIV IS_DIV)))
(defun   (signed_inst)   (force-bin (+ IS_SMOD IS_SDIV)))

;; bit decompositions of the most significant bytes
(defun   (bit-dec-msb1)   (+ (* 128 (shift MSB_1 -7))
                             (* 64  (shift MSB_1 -6))
                             (* 32  (shift MSB_1 -5))
                             (* 16  (shift MSB_1 -4))
                             (* 8   (shift MSB_1 -3))
                             (* 4   (shift MSB_1 -2))
                             (* 2   (shift MSB_1 -1))
                             MSB_1))

(defun   (bit-dec-msb2)   (+ (* 128 (shift MSB_2 -7))
                             (* 64  (shift MSB_2 -6))
                             (* 32  (shift MSB_2 -5))
                             (* 16  (shift MSB_2 -4))
                             (* 8   (shift MSB_2 -3))
                             (* 4   (shift MSB_2 -2))
                             (* 2   (shift MSB_2 -1))
                             MSB_2))

(defun (set-negative zHi zLo yHi yLo)
  (if-not-zero yLo
               (begin (eq! zHi (- THETA2 yHi 1))
                      (eq! zLo (- THETA2 yLo)))
               (begin (vanishes! zLo)
                      (if-zero (* yHi (- THETA_SQUARED_OVER_TWO yHi))
                               (eq! zHi yHi)
                               (eq! zHi (- THETA2 yHi))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    5.5 instruction decoding    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint set-flag-sum ()
  (eq! (flag_sum) (~ STAMP)))

(defconstraint instruction-decoding ()
  (eq! INST (weight_sum)))

(defconstraint signed-inst ()
  (eq! SIGNED (signed_inst)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    5.6 OLI and MLI decoding    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint oli-and-mli-exclusivity ()
  (eq! (force-bin (+ OLI MLI))
       (flag_sum)))

(defconstraint set-oli-and-mli (:guard STAMP)
  (if-zero ARG_2_HI
           (if-zero ARG_2_LO
                    (eq! OLI 1)
                    (eq! MLI 1))
           (eq! MLI 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    5.7 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0}) ;; ""
  (vanishes! STAMP))

(defconstraint heartbeat ()
  (begin (or! (will-remain-constant! STAMP) (will-inc! STAMP 1))
         (if-not-zero (will-remain-constant! STAMP)
                      (vanishes! (next CT)))
         (if-eq OLI 1 (will-inc! STAMP 1))
         (if-eq MLI 1
                (if-eq-else CT MMEDIUMMO (will-inc! STAMP 1) (will-inc! CT 1)))))

(defconstraint last-row (:domain {-1}) ;; ""
  (if-eq MLI 1 (eq! CT MMEDIUMMO)))

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
;;    5.9 Binary, bytehood and byte decomposition constraints    ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

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
;;    5.10 Auto Vanishing    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint oli-imply-trivial-result ()
  (if-eq OLI 1
         (begin (vanishes! RES_HI)
                (vanishes! RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    5.12.2 Absolute values    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (set-absolute-value a_hi a_lo x_hi x_lo sgn)
  (if-zero sgn
           (begin (eq! a_hi x_hi)
                  (eq! a_lo x_lo))
           (begin (if-zero x_lo
                           (begin (eq! a_hi (- THETA2 x_hi))
                                  (vanishes! a_lo))
                           (begin (eq! a_hi (- THETA2 (+ x_hi 1)))
                                  (eq! a_lo (- THETA2 x_lo)))))))

(defconstraint set-absolute-values ()
  (if-eq CT MMEDIUMMO
         (begin (set-absolute-value (ABS_1_HI) (ABS_1_LO) ARG_1_HI ARG_1_LO (* SIGNED (sgn_1)))
                (set-absolute-value (ABS_2_HI) (ABS_2_LO) ARG_2_HI ARG_2_LO (* SIGNED (sgn_2))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    5.12.3 target constraints    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint target-constraints (:guard STAMP)
  (if-eq CT MMEDIUMMO
         (begin (eq! ARG_1_HI
                   (+ (* THETA (ARG1_3)) (ARG1_2)))
                (eq! ARG_2_HI
                   (+ (* THETA (ARG2_3)) (ARG2_2)))
                ;
                (eq! (shift BYTE_1_3 -7) (bit-dec-msb1))
                (eq! (shift BYTE_2_3 -7) (bit-dec-msb2))
                ;
                (eq! (+ (* (B_0) (Q_1)) (* (B_1) (Q_0)))
                   (+ (* THETA2 (alpha)) (* THETA (H_1)) (H_0)))
                (eq! (+ (* (B_0) (Q_3)) (* (B_1) (Q_2)) (* (B_2) (Q_1)) (* (B_3) (Q_0)))
                   (H_2))
                (vanishes! (+ (* (B_1) (Q_3))
                              (* (B_2) (Q_2))
                              (* (B_3) (Q_1))
                              (* (B_2) (Q_3))
                              (* (B_3) (Q_2))
                              (* (B_3) (Q_3)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    5.12.4 comp constraint    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint comparison-constraint ()
  (if-eq CT MMEDIUMMO
         (begin (eq! (+ (Delta_3) (lt_3))
                     (* (- (* 2 (lt_3)) 1)
                        (- (B_3) (R_3))))
                (eq! (+ (Delta_2) (lt_2))
                     (* (- (* 2 (lt_2)) 1)
                        (- (B_2) (R_2))))
                (eq! (+ (Delta_1) (lt_1))
                     (* (- (* 2 (lt_1)) 1)
                        (- (B_1) (R_1))))
                (eq! (+ (Delta_0) (lt_0))
                     (* (- (* 2 (lt_0)) 1)
                        (- (B_0) (R_0))))
                (if-eq-else (B_3) (R_3) (eq! (eq_3) 1) (vanishes! (eq_3)))
                (if-eq-else (B_2) (R_2) (eq! (eq_2) 1) (vanishes! (eq_2)))
                (if-eq-else (B_1) (R_1) (eq! (eq_1) 1) (vanishes! (eq_1)))
                (if-eq-else (B_0) (R_0) (eq! (eq_0) 1) (vanishes! (eq_0)))
                (eq! 1
                   (+ (lt_3)
                      (* (eq_3) (lt_2))
                      (* (eq_3) (eq_2) (lt_1))
                      (* (eq_3) (eq_2) (eq_1) (lt_0))))
                (eq! (+ (* THETA2 (shift CMP_2 -3))
                        (* THETA ACC_H_1)
                        ACC_H_0)
                     (+ (* ACC_B_0 ACC_Q_1) (* ACC_B_1 ACC_Q_0)))
                (eq! ACC_H_2
                     (+ (* ACC_B_0 ACC_Q_3) (* ACC_B_1 ACC_Q_2) (* ACC_B_2 ACC_Q_1) (* ACC_B_3 ACC_Q_0))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    5.12.5 result constraint  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint mod-result (:guard IS_MOD)
  (if-eq CT MMEDIUMMO
         (begin (eq! RES_HI (R_HI))
                (eq! RES_LO (R_LO)))))

(defconstraint div-result (:guard IS_DIV)
  (if-eq CT MMEDIUMMO
         (begin (eq! RES_HI (Q_HI))
                (eq! RES_LO (Q_LO)))))

(defconstraint smod-result (:guard IS_SMOD)
  (if-eq CT MMEDIUMMO
         (if-zero (sgn_1)
                  (begin (eq! RES_HI (R_HI))
                         (eq! RES_LO (R_LO)))
                  (set-negative RES_HI RES_LO (R_HI) (R_LO)))))

(defconstraint sdiv-result (:guard IS_SDIV)
  (if-eq CT MMEDIUMMO
         (if-eq-else (sgn_1) (sgn_2)
                     (begin (eq! RES_HI (Q_HI))
                            (eq! RES_LO (Q_LO)))
                     (set-negative RES_HI RES_LO (Q_HI) (Q_LO)))))
