(module shf)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; ;; 2.1.1
;; (defconstraint    first-row (:domain {0})
;;                   (vanishes! STAMP))

;; 2.1.2
(defconstraint    stamp-increments ()
                  (or! (will-inc! STAMP 0) (will-inc! STAMP 1)))

;; 2.1.3 and 4
(defconstraint    zero-row ()
                  (if-zero STAMP
                           (vanishes! IOMF)
                           (eq!       IOMF 1)))

;; 2.1.5
(defconstraint    counter-reset ()
                  (if-not (will-remain-constant! STAMP)
                          (vanishes! (shift CT 1))))

;; 2.1.6
(defconstraint    INST-inside-and-outside-of-padding ()
                  (if-zero    IOMF
                              (vanishes!    INST)
                              (or!    (eq!    INST    EVM_INST_SHL)
                                      (eq!    INST    EVM_INST_SHR)
                                      (eq!    INST    EVM_INST_SAR))))

;; 2.1.7
(defconstraint    heartbeat ()
                  (if-not-zero IOMF
                                 (if-not-zero OLI
                                              ;; 2.1.5.b
                                              ;; OLI == 1
                                              (will-inc!    STAMP    1)
                                              ;; 2.1.5.c
                                              ;; OLI == 0
                                              (if-eq-else CT LLARGEMO
                                                          ;; 2.1.5.c.ii
                                                          ;; If CT == LLARGEMO (15)
                                                          (will-inc!    STAMP    1)
                                                          ;; 2.1.5.c.i
                                                          ;; If CT != LLARGEMO (15)
                                                          (begin (will-inc!    CT    1)
                                                                 (will-remain-constant! OLI))))))

;; 2.1.8
(defconstraint last-row (:domain {-1})
               (if-not-zero STAMP
                            (if-zero OLI
                                     (eq! CT LLARGEMO))))

;; counter-constancy constraints
(defun (counter-constancy ct X)
  (if-not-zero ct
               (remained-constant! X)))

;; counter-constant columns
(defconstraint counter-constancies ()
               (begin (counter-constancy CT BIT_B_3)
                      (counter-constancy CT BIT_B_4)
                      (counter-constancy CT BIT_B_5)
                      (counter-constancy CT BIT_B_6)
                      (counter-constancy CT BIT_B_7)
                      (counter-constancy CT NEG)
                      (counter-constancy CT SHD)
                      (counter-constancy CT LOW_3)
                      (counter-constancy CT µSHP)
                      (counter-constancy CT OLI)
                      (counter-constancy CT KNOWN)))

;; stamp constancies
(defconstraint stamp-constancies ()
               (begin (stamp-constancy STAMP ARG_1_HI)
                      (stamp-constancy STAMP ARG_1_LO)
                      (stamp-constancy STAMP ARG_2_HI)
                      (stamp-constancy STAMP ARG_2_LO)
                      (stamp-constancy STAMP RES_HI)
                      (stamp-constancy STAMP RES_LO)
                      (stamp-constancy STAMP INST)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    2.3 binary columns   ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; 2.2.2 SHD constraints
(defconstraint shift_direction_constraint (:guard STAMP)
               (if-eq-else INST EVM_INST_SHL
                           ;; INST == SHL => SHD = 0
                           (vanishes! SHD)
                           ;; INST != SHL => SHD = 1
                           (eq! SHD 1)))

;; 2.2.3 OLI constraints
(defconstraint oli_constraints (:guard STAMP)
               (if-zero (* (- INST EVM_INST_SAR) ARG_1_HI)
                        (vanishes! OLI)
                        (eq! OLI 1)))

;; 2.2.4 BITS constraints
(defconstraint bits_constraints (:guard STAMP)
               (if-zero OLI
                        (begin (if-zero CT
                                        (begin (eq! NEG BITS)
                                               (eq! BYTE_2
                                                  (+ (* 128 BITS)
                                                     (* 64 (shift BITS 1))
                                                     (* 32 (shift BITS 2))
                                                     (* 16 (shift BITS 3))
                                                     (* 8 (shift BITS 4))
                                                     (* 4 (shift BITS 5))
                                                     (* 2 (shift BITS 6))
                                                     (shift BITS 7))))
                                        (if-eq CT LLARGEMO
                                               (eq! BYTE_1
                                                  (+ (* 128 (shift BITS -7))
                                                     (* 64 (shift BITS -6))
                                                     (* 32 (shift BITS -5))
                                                     (* 16 (shift BITS -4))
                                                     (* 8 (shift BITS -3))
                                                     (* 4 (shift BITS -2))
                                                     (* 2 (shift BITS -1))
                                                     BITS)))))))

(defconstraint setting_stuff ()
               (if-eq CT LLARGEMO
                      (begin (eq! LOW_3
                                (+ (* 4 (shift BITS -2))
                                   (* 2 (shift BITS -1))
                                   BITS))
                             (if-zero SHD
                                      (eq! µSHP (- 8 LOW_3))
                                      (eq! µSHP LOW_3))
                             (eq! BIT_B_3 (shift BITS -3))
                             (eq! BIT_B_4 (shift BITS -4))
                             (eq! BIT_B_5 (shift BITS -5))
                             (eq! BIT_B_6 (shift BITS -6))
                             (eq! BIT_B_7 (shift BITS -7)))))

;; 2.2.5 KNOWN constraints
(defconstraint known_constraint ()
               (if-eq CT LLARGEMO
                      (if-eq-else INST EVM_INST_SAR
                                  (if-zero ARG_1_HI
                                           (if-eq-else ARG_1_LO BYTE_1 (vanishes! KNOWN) (eq! KNOWN 1))
                                           (eq! KNOWN 1))
                                  (if-eq-else ARG_1_LO BYTE_1 (vanishes! KNOWN) (eq! KNOWN 1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.3 byte decompositions   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (byte-decomposition ct acc bytes)
  (if-zero ct
           (eq! acc bytes)
           (eq! acc
                (+ (* 256 (shift acc -1))
                   bytes))))

;; byte decompositions
(defconstraint byte_decompositions ()
               (begin (byte-decomposition CT ACC_1 BYTE_1)
                      (byte-decomposition CT ACC_2 BYTE_2)
                      (byte-decomposition CT ACC_3 BYTE_3)
                      (byte-decomposition CT ACC_4 BYTE_4)
                      (byte-decomposition CT ACC_5 BYTE_5)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.4 target constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target_constraints ()
               (if-eq CT LLARGEMO
                      (begin (eq! ACC_1 ARG_1_LO)
                             (eq! ACC_2 ARG_2_HI)
                             (eq! ACC_3 ARG_2_LO)
                             (eq! ACC_4 RES_HI)
                             (eq! ACC_5 RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.5 shifting constraints   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (left-shift-by k ct bit_b (bit_n :binary) B1_init B2_init B1_shft B2_shft)
  (begin (plateau-constraint ct bit_n (- LLARGE k))
         (if-zero bit_b
                  (begin (eq! B1_shft B1_init)
                         (eq! B2_shft B2_init))
                  (if-zero bit_n
                           (begin (eq! B1_shft (shift B1_init k))
                                  (eq! B2_shft (shift B2_init k)))
                           (begin (eq! B1_shft
                                     (shift B2_init (- k LLARGE)))
                                  (vanishes! B2_shft))))))

(defun (right-shift-by k ct neg inst bit_b (bit_n :binary) B1_init B2_init B1_shft B2_shft)
  (begin (plateau-constraint ct bit_n k)
         (if-zero bit_b
                  (begin (eq! B1_shft B1_init)
                         (eq! B2_shft B2_init))
                  (if-zero bit_n
                           ;; bit_n == 0
                           (begin (if-eq-else inst EVM_INST_SAR
                                              ;; INST == SAR
                                              (eq! B1_shft (* 255 neg))
                                              ;; INST != SAR
                                              (vanishes! B1_shft))
                                  (eq! B2_shft
                                     (shift B1_init (- LLARGE k))))
                           ;; bit_n == 1
                           (begin (eq! B1_shft
                                     (shift B1_init (- 0 k)))
                                  (eq! B2_shft
                                     (shift B2_init (- 0 k))))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    2.6 shifted bytes constraints   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (shb_3)
  (if-zero SHD
           ;; SHD == 0
           (if-eq-else CT LLARGEMO
                       ;; CT == 15
                       (begin (eq! SHB_3_HI
                                 (+ LA_HI
                                    (shift RA_LO (- 0 LLARGEMO))))
                              (eq! SHB_3_LO LA_LO))
                       ;; CT != 15
                       (begin (eq! SHB_3_HI
                                 (+ LA_HI (shift RA_HI 1)))
                              (eq! SHB_3_LO
                                 (+ LA_LO (shift RA_LO 1)))))
           ;; SHD == 1
           (if-zero CT
                    ;; CT == 0
                    (begin (if-not-zero (- INST EVM_INST_SHR)
                                        (eq! SHB_3_HI
                                           (+ (* NEG ONES) RA_HI)))
                           (if-not-zero (- INST EVM_INST_SAR)
                                        (eq! SHB_3_HI RA_HI))
                           (eq! SHB_3_LO
                              (+ (shift LA_HI LLARGEMO) RA_LO)))
                    ;; CT != 0
                    (begin (eq! SHB_3_HI
                              (+ (shift LA_HI (- 0 1))
                                 RA_HI))
                           (eq! SHB_3_LO
                              (+ (shift LA_LO (- 0 1))
                                 RA_LO))))))

(defun (shb_4)
  (if-zero SHD
           (left-shift-by 1 CT BIT_B_3 BIT_1 SHB_3_HI SHB_3_LO SHB_4_HI SHB_4_LO)
           (right-shift-by 1 CT NEG INST BIT_B_3 BIT_1 SHB_3_HI SHB_3_LO SHB_4_HI SHB_4_LO)))

(defun (shb_5)
  (if-zero SHD
           (left-shift-by 2 CT BIT_B_4 BIT_2 SHB_4_HI SHB_4_LO SHB_5_HI SHB_5_LO)
           (right-shift-by 2 CT NEG INST BIT_B_4 BIT_2 SHB_4_HI SHB_4_LO SHB_5_HI SHB_5_LO)))

(defun (shb_6)
  (if-zero SHD
           (left-shift-by 4 CT BIT_B_5 BIT_3 SHB_5_HI SHB_5_LO SHB_6_HI SHB_6_LO)
           (right-shift-by 4 CT NEG INST BIT_B_5 BIT_3 SHB_5_HI SHB_5_LO SHB_6_HI SHB_6_LO)))

(defun (shb_7)
  (if-zero SHD
           (left-shift-by 8 CT BIT_B_6 BIT_4 SHB_6_HI SHB_6_LO SHB_7_HI SHB_7_LO)
           (right-shift-by 8 CT NEG INST BIT_B_6 BIT_4 SHB_6_HI SHB_6_LO SHB_7_HI SHB_7_LO)))

(defun (res_bytes)
  (if-zero KNOWN
           ;; KNOWN == 0
           (if-zero SHD
                    ;; SHD == 0
                    (if-zero BIT_B_7
                             (begin (eq! BYTE_4 SHB_7_HI)
                                    (eq! BYTE_5 SHB_7_LO))
                             (begin (eq! BYTE_4 SHB_7_LO)
                                    (vanishes! BYTE_5)))
                    ;; SHD == 1
                    (if-zero BIT_B_7
                             (begin (eq! BYTE_4 SHB_7_HI)
                                    (eq! BYTE_5 SHB_7_LO))
                             (begin (if-eq-else INST EVM_INST_SHR
                                                (vanishes! BYTE_4)
                                                (eq! BYTE_4 (* 255 NEG)))
                                    (eq! BYTE_5 SHB_7_HI))))
           ;; KNOWN == 1
           (if-eq-else INST EVM_INST_SAR
                       ;; INST == SAR
                       (begin (eq! BYTE_4 (* 255 NEG))
                              (eq! BYTE_5 (* 255 NEG)))
                       ;; INST != SAR
                       (begin (vanishes! BYTE_4)
                              (vanishes! BYTE_5)))))

;; all shift constraints
(defconstraint shifted_byte_columns ()
               (if-not-zero STAMP
                            (if-zero OLI
                                     (begin (shb_3)
                                            (shb_4)
                                            (shb_5)
                                            (shb_6)
                                            (shb_7)
                                            (res_bytes)))))


