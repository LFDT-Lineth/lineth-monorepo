(module exp)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.1 Shorthands           ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (flag_sum_perspective)   (+ CMPTN MACRO PRPRC))
(defun (flag_sum_macro)         (+ IS_EXP_LOG IS_MODEXP_LOG))
(defun (wght_sum_macro)         (+ (* EXP_INST_EXPLOG       IS_EXP_LOG)
                                   (* EXP_INST_MODEXPLOG    IS_MODEXP_LOG)))
(defun (maxct_sum)              (+ (* CMPTN (+ (* CT_MAX_CMPTN_EXP_LOG    IS_EXP_LOG) (* CT_MAX_CMPTN_MODEXP_LOG    IS_MODEXP_LOG)))
                                   (* MACRO (+ (* CT_MAX_MACRO_EXP_LOG    IS_EXP_LOG) (* CT_MAX_MACRO_MODEXP_LOG    IS_MODEXP_LOG)))
                                   (* PRPRC (+ (* CT_MAX_PRPRC_EXP_LOG    IS_EXP_LOG) (* CT_MAX_PRPRC_MODEXP_LOG    IS_MODEXP_LOG)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 binary constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; binary columns are already :binary@prove
;; and both shorthands (flag_sum_perspective) and (flag_sum_macro)
;; are de facto binary

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.3 Flag sum perspectives  ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; (flag_sum_perspective) is thus binary by construction
(defconstraint   flag-sum-perspective-padding-non-padding ()
                 (if-zero STAMP
                          (eq! (flag_sum_perspective) 0)
                          (eq! (flag_sum_perspective) 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.4 Instruction decoding   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; (flag_sum_macro) is thus binary by construction
(defconstraint   instruction-decoding-padding-non-padding ()
                 (if-zero STAMP
                          (eq! (flag_sum_macro) 0)
                          (eq! (flag_sum_macro) 1)))

(defconstraint   instruction-decoding-exp-inst (:perspective macro)
                 (eq! EXP_INST (wght_sum_macro)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.5 Constancies          ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   stamp-constancy ()
                 (begin (stamp-constancy STAMP IS_EXP_LOG)
                        (stamp-constancy STAMP IS_MODEXP_LOG)))

(defconstraint   counter-constancy ()
                 (begin (counter-constancy CT CMPTN)
                        (counter-constancy CT MACRO)
                        (counter-constancy CT PRPRC)))

(defconstraint   computation-constancy (:perspective computation)
                 (begin (perspective-constancy CMPTN PLT_JMP)
                        (perspective-constancy CMPTN MSB)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    3.2 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;; 1
(defconstraint   first-row (:domain {0}) (vanishes! STAMP)) ;; ""

;; 2
(defconstraint   stamp-vanishing-values ()
                 (if-zero STAMP
                          (vanishes! CT)))

;; 3
(defconstraint   stamp-increments ()
                 (eq! (next STAMP)
                      (+ STAMP
                         (* (next CMPTN) (- 1 CMPTN)))))

;; 4
(defconstraint   maxct ()
                 (eq! CT_MAX (maxct_sum)))

;; 5
(defconstraint   disallowed-transitions ()
                 (vanishes! (+ (* CMPTN (next PRPRC))
                               (* MACRO
                                  (+ (next MACRO) (next CMPTN)))
                               (* PRPRC (next MACRO)))))

;; 6
(defconstraint   allowed-transitions ()
                 (if-not-zero STAMP
                              (if-eq CT CT_MAX
                                     (eq! (+ (* CMPTN (next MACRO))
                                             (* MACRO (next PRPRC))
                                             (* PRPRC (next CMPTN)))
                                          1))))

;; 7
(defconstraint   instruction-counter-cycle ()
                 (if-eq-else CT CT_MAX
                             (eq! (next CT) 0)
                             (will-inc! CT 1)))

;; 8
(defconstraint   finalization (:domain {-1}) ;; ""
                 (if-not-zero STAMP
                              (begin (eq! PRPRC 1)
                                     (eq! CT CT_MAX))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.7 Byte decomposition   ;;
;;        constraints          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   byte-decompositions (:perspective computation)
                 (begin (byte-decomposition CT RAW_ACC RAW_BYTE)
                        (byte-decomposition CT TRIM_ACC TRIM_BYTE)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.8 Bit decomposition    ;;
;;        constraints          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   bit-decompositions (:perspective computation :guard IS_MODEXP_LOG)
                 (bit-decomposition CT MSB_ACC MSB_BIT))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.9 Plateau bit          ;;
;;        constraints          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   plateau-constraints (:perspective computation :guard IS_MODEXP_LOG)
                 (plateau-constraint CT PLT_BIT PLT_JMP))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    3.10 Counting nonzeroness  ;;
;;         constraints           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; non zero bit constraint
(defpurefun (non-zero-bit x nzb)
            (if-zero x
                     (vanishes! nzb)
                     (eq! nzb 1)))

;; counting nonzeroness constraint
(defpurefun (counting-nonzeroness ct nzb_acc nzb)
            (if-zero ct
                     (eq! nzb_acc nzb)
                     (eq! nzb_acc
                          (+ (prev nzb_acc) nzb))))

(defconstraint   filter-raw (:perspective computation)
                 (eq! TRIM_BYTE
                      (* RAW_BYTE (- 1 PLT_BIT))))

(defconstraint   counting-nonzeroness-trim (:perspective computation)
                 (begin (non-zero-bit TRIM_ACC TANZB)
                        (counting-nonzeroness CT TANZB_ACC TANZB)))

(defconstraint   counting-nonzeroness-msb (:perspective computation)
                 (begin (non-zero-bit MSB_ACC MANZB)
                        (counting-nonzeroness CT MANZB_ACC MANZB)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    3.11 Most significant           ;;
;;         byte constraints           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   most-significant-byte-start (:perspective computation :guard IS_MODEXP_LOG)
                 (if-zero CT
                          (if-not-zero TANZB_ACC
                                       (eq! MSB TRIM_BYTE))))

(defconstraint   most-significant-byte-middle (:perspective computation :guard IS_MODEXP_LOG)
                 (if-not-zero CT
                              (if-not-zero TANZB_ACC
                                           (if-zero (prev TANZB_ACC)
                                                    (eq! MSB TRIM_BYTE)))))

(defconstraint   most-significant-byte-end (:perspective computation :guard IS_MODEXP_LOG)
                 (if-eq CT CT_MAX
                        (if-zero TANZB_ACC
                                 (vanishes! MSB))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    3.12 Word comparisons utilities ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (callToLT k a b c d) (begin (eq!   (shift preprocessing/WCP_FLAG       k) 1)
                                   (eq!   (shift preprocessing/WCP_INST       k) EVM_INST_LT)
                                   (eq!   (shift preprocessing/WCP_ARG_1_HI   k) a)
                                   (eq!   (shift preprocessing/WCP_ARG_1_LO   k) b)
                                   (eq!   (shift preprocessing/WCP_ARG_2_HI   k) c)
                                   (eq!   (shift preprocessing/WCP_ARG_2_LO   k) d)))

(defun (callToISZERO k a b) (begin (eq!                (shift preprocessing/WCP_FLAG       k) 1)
                                   (eq!                (shift preprocessing/WCP_INST       k) EVM_INST_ISZERO)
                                   (eq!                (shift preprocessing/WCP_ARG_1_HI   k) a)
                                   (eq!                (shift preprocessing/WCP_ARG_1_LO   k) b)
                                   (debug (vanishes!   (shift preprocessing/WCP_ARG_2_HI   k)))
                                   (debug (vanishes!   (shift preprocessing/WCP_ARG_2_LO   k)))))

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    4 EXP_LOG       ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    4.2 Shorthands  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defun (exponent_hi)          [ macro/DATA 1 ])
(defun (exponent_lo)          [ macro/DATA 2 ])
(defun (dyn_cost)             [ macro/DATA 5 ]) ;; ""
(defun (exponent_byte_length) (prev computation/TANZB_ACC))

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;    4.3 Preprocessing ;;
;;                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   exp-log---preprocessing---exponent-hi-is-zero (:perspective macro :guard IS_EXP_LOG)
                 (callToISZERO 1 0 (exponent_hi)))

(defun (expn_hi_is_zero) (next preprocessing/WCP_RES))

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;    4.4 Linking       ;;
;;        constraints   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   exp-log---linking-constraints (:perspective macro :guard IS_EXP_LOG)
                 (begin (eq! (shift computation/PLT_JMP -1) 16)
                        (if-not-zero (expn_hi_is_zero)
                                     (eq! (shift computation/RAW_ACC -1) (exponent_lo)))
                        (if-zero (expn_hi_is_zero)
                                 (eq! (shift computation/RAW_ACC -1) (exponent_hi)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    4.5 Justify          ;;
;;        hub prediction   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   exp-log---justify-hub-prediction (:perspective macro :guard IS_EXP_LOG)
                 (if-zero (expn_hi_is_zero)
                          (eq! (dyn_cost)
                               (* GAS_CONST_G_EXP_BYTE (+ (exponent_byte_length) 16)))
                          (eq! (dyn_cost) (* GAS_CONST_G_EXP_BYTE (exponent_byte_length)))))

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    5 MODEXP_LOG    ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    5.2 Shorthands  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defun (raw_lead_hi)                                 [macro/DATA 1])
(defun (raw_lead_lo)                                 [macro/DATA 2])
(defun (cds_cutoff)                                  [macro/DATA 3])
(defun (ebs_cutoff)                                  [macro/DATA 4])
(defun (lead_log)                                    [macro/DATA 5]) ;; ""
(defun (trim_acc)                                    (shift    computation/TRIM_ACC     -1))
(defun (nbytes_excluding_leading_byte)               (shift    computation/TANZB_ACC    -2))
(defun (nbits_of_leading_byte_excluding_leading_bit) (shift    computation/MANZB_ACC    -2))
(defun (padded_base_2_log)                           (+   (*   8 (nbytes_excluding_leading_byte))
                                                          (nbits_of_leading_byte_excluding_leading_bit)))

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;    5.3 Preprocessing ;;
;;                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

;; 1
(defconstraint   modexp-log---preprocessing---cds-cutoff-less-than-ebs-cutoff (:perspective macro :guard IS_MODEXP_LOG)
                 (callToLT 1 0 (cds_cutoff) 0 (ebs_cutoff)))

(defun (comp)       (shift preprocessing/WCP_RES 1))
(defun (min_cutoff) (+ (* (cds_cutoff) (comp))
                       (* (ebs_cutoff) (- 1 (comp)))))

;; 2
(defconstraint   modexp-log---preprocessing---min-cutoff-less-or-equal-than-16 (:perspective macro :guard IS_MODEXP_LOG)
                 (callToLT 2 0 (min_cutoff) 0 17))

(defun (min_cutoff_leq_16) (shift preprocessing/WCP_RES 2))

;; 3
(defconstraint   modexp-log---preprocessing---raw-lead-hi-is-zero (:perspective macro :guard IS_MODEXP_LOG)
                 (callToISZERO 3 0 (raw_lead_hi)))

(defun (raw_lead_hi_is_zero) (shift preprocessing/WCP_RES 3))

;; 4
(defconstraint   modexp-log---preprocessing---trim-acc-is-zero (:perspective macro :guard IS_MODEXP_LOG)
                 (callToISZERO 4 0 (trim_acc)))

(defun (trim_acc_is_zero) (shift preprocessing/WCP_RES 4))

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;    5.4 Linking       ;;
;;        constraints   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   modexp-log---linking-constraints (:perspective macro :guard IS_MODEXP_LOG)
                 (begin (if-not-zero (min_cutoff_leq_16)
                                     (begin (eq! (shift computation/RAW_ACC -1) (raw_lead_hi))
                                            (eq! (shift computation/PLT_JMP -1) (min_cutoff)))
                                     (begin (if-not-zero (raw_lead_hi_is_zero)
                                                         (begin (eq! (shift computation/RAW_ACC -1) (raw_lead_lo))
                                                                (eq! (shift computation/PLT_JMP -1) (- (min_cutoff) 16)))
                                                         (begin (eq! (shift computation/RAW_ACC -1) (raw_lead_hi))
                                                                (eq! (shift computation/PLT_JMP -1) 16)))))
                        (eq! (prev computation/MSB) (prev computation/MSB_ACC))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    5.5 Justify          ;;
;;        hub prediction   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   modexp-log---justify-hub-prediction (:perspective macro :guard IS_MODEXP_LOG)
                 (if-not-zero    (min_cutoff_leq_16)
                                 (if-not-zero    (trim_acc_is_zero)
                                                 (vanishes! (lead_log))
                                                 (eq!       (lead_log)
                                                            (- (padded_base_2_log) (* 8 (- 16 (ebs_cutoff))))))
                                 (if-not-zero    (raw_lead_hi_is_zero)
                                                 (if-not-zero    (trim_acc_is_zero)
                                                                 (vanishes! (lead_log))
                                                                 (eq!       (lead_log)
                                                                            (- (padded_base_2_log) (* 8 (- 32 (ebs_cutoff))))))
                                                 (eq! (lead_log)
                                                      (+ (padded_base_2_log) (* 8 (- (ebs_cutoff) 16)))))))
