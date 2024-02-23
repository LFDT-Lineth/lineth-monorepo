(module mmu)

(defun (flag-sum)
  (+ MACRO PRPRC MICRO))

(defconstraint perspective-flag ()
  (begin (debug (is-binary (flag-sum)))
         (if-zero STAMP
                  (vanishes! (flag-sum))
                  (eq! (flag-sum) 1))))

;;
;; Heartbeat
;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint no-macrostamp-no-microstamp ()
  (if-zero STAMP
           (vanishes! MMIO_STAMP)))

(defconstraint mmu-stamp-evolution ()
  (did-inc! STAMP MACRO))

(defconstraint mmio-stamp-evolution ()
  (did-inc! MMIO_STAMP MICRO))

(defconstraint prprc-after-macro (:guard MACRO)
  (eq! (next PRPRC) 1))

(defconstraint after-prprc (:guard PRPRC)
  (begin (debug (eq! (+ (next PRPRC) (next MICRO))
                     1))
         (if-zero prprc/CT
                  (will-eq! MICRO 1)
                  (begin (will-dec! prprc/CT 1)
                         (will-eq! PRPRC 1)))))

(defconstraint tot-nb-of-micro-inst ()
  (eq! TOT (+ TOTLZ TOTNT TOTRZ)))

(defconstraint after-micro (:guard MICRO)
  (begin (debug (eq! (+ (next MICRO) (next MACRO))
                     1))
         (did-dec! TOT 1)
         (if-zero TOT
                  (begin (will-eq! MACRO 1)
                         (debug (vanishes! TOTLZ))
                         (debug (vanishes! TOTNT))
                         (debug (vanishes! TOTRZ)))
                  (will-eq! MICRO 1))
         (if-zero (prev TOTLZ)
                  (vanishes! TOTLZ)
                  (did-dec! TOTLZ 1))
         (if-zero (prev TOTNT)
                  (vanishes! TOTNT)
                  (did-dec! (+ TOTLZ TOTNT) 1))))

(defconstraint last-row (:domain {-1})
  (if-not-zero STAMP
               (begin (eq! MICRO 1)
                      (vanishes! TOT))))

;;
;; Constancies
;;
(defun (prprc-constant X)
  (if-eq PRPRC 1 (remained-constant! X)))

(defconstraint prprc-constancies ()
  (begin (prprc-constant TOT)
         (debug (prprc-constant TOTLZ))
         (debug (prprc-constant TOTNT))
         (debug (prprc-constant TOTRZ))))

(defun (stamp-decrementing X)
  (if-not-zero (- STAMP
                  (+ (prev STAMP) 1))
               (any! (remained-constant! X) (did-inc! X 1))))

(defconstraint stamp-decrementings ()
  (begin (stamp-decrementing TOT)
         (stamp-decrementing TOTLZ)
         (stamp-decrementing TOTNT)
         (stamp-decrementing TOTRZ)))

(defun (stamp-constant X)
  (if-not-zero (- STAMP
                  (+ (prev STAMP) 1))
               (remained-constant! X)))

(defconstraint stamp-constancies ()
  (begin (for i [5] (stamp-constant [OUT 1]))
         (for i [5] (stamp-constant [BIN 1]))
         (stamp-constant (bin-flag-sum))))

(defun (micro-instruction-writing-constant X)
  (if-eq MICRO 1
         (if-eq (prev MICRO) 1 (remained-constant! X))))

(defconstraint mmio-row-constancies ()
  (begin (micro-instruction-writing-constant micro/CN_S)
         (micro-instruction-writing-constant micro/CN_T)
         (micro-instruction-writing-constant micro/SUCCESS_BIT)
         (micro-instruction-writing-constant micro/EXO_SUM)
         (micro-instruction-writing-constant micro/PHASE)
         (micro-instruction-writing-constant micro/EXO_ID)
         (micro-instruction-writing-constant micro/KEC_ID)
         (micro-instruction-writing-constant micro/TOTAL_SIZE)))

;;
;; Instruction Decoding
;;
(defun (bin-flag-sum)
  (+ (* 1 IS_MLOAD)
     (* 2 IS_MSTORE)
     (* 3 IS_MSTORE8)
     (* 4 IS_INVALID_CODE_PREFIX)
     (* 5 IS_RIGHT_PADDED_WORD_EXTRACTION)
     (* 6 IS_RAM_TO_EXO_WITH_PADDING)
     (* 7 IS_EXO_TO_RAM_TRANSPLANTS)
     (* 8 IS_RAM_TO_RAM_SANS_PADDING)
     (* 9 IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA)
     (* 10 IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)
     (* 11 IS_MODEXP_ZERO)
     (* 12 IS_MODEXP_DATA)
     (* 13 IS_BLAKE)))

(defun (is-any-to-ram-with-padding)
  (+ IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))

(defun (inst-flag-sum)
  (+ IS_MLOAD
     IS_MSTORE
     IS_MSTORE8
     IS_INVALID_CODE_PREFIX
     IS_RIGHT_PADDED_WORD_EXTRACTION
     IS_RAM_TO_EXO_WITH_PADDING
     IS_EXO_TO_RAM_TRANSPLANTS
     IS_RAM_TO_RAM_SANS_PADDING
     (is-any-to-ram-with-padding)
     IS_MODEXP_ZERO
     IS_MODEXP_DATA
     IS_BLAKE))

(defun (weight-flag-sum)
  (+ (* MMU_INST_MLOAD IS_MLOAD)
     (* MMU_INST_MSTORE IS_MSTORE)
     (* MMU_INST_MSTORE8 IS_MSTORE8)
     (* MMU_INST_INVALID_CODE_PREFIX IS_INVALID_CODE_PREFIX)
     (* MMU_INST_RIGHT_PADDED_WORD_EXTRACTION IS_RIGHT_PADDED_WORD_EXTRACTION)
     (* MMU_INST_RAM_TO_EXO_WITH_PADDING IS_RAM_TO_EXO_WITH_PADDING)
     (* MMU_INST_EXO_TO_RAM_TRANSPLANTS IS_EXO_TO_RAM_TRANSPLANTS)
     (* MMU_INST_RAM_TO_RAM_SANS_PADDING IS_RAM_TO_RAM_SANS_PADDING)
     (* MMU_INST_ANY_TO_RAM_WITH_PADDING (is-any-to-ram-with-padding))
     (* MMU_INST_MODEXP_ZERO IS_MODEXP_ZERO)
     (* MMU_INST_MODEXP_DATA IS_MODEXP_DATA)
     (* MMU_INST_BLAKE IS_BLAKE)))

(defconstraint inst-flag-is-one ()
  (eq! (inst-flag-sum) (flag-sum)))

(defconstraint set-inst-flag (:guard MACRO)
  (eq! (weight-flag-sum) macro/INST))

;;
;; Micro Instruction writing row types
;;
(defun (ntrv-row)
  (+ NT_ONLY NT_FIRST NT_MDDL NT_LAST))

(defun (rzro-row)
  (+ RZ_ONLY RZ_FIRST RZ_MDDL RZ_LAST))

(defun (zero-row)
  (+ LZRO (rzro-row)))

(defconstraint sum-row-flag ()
  (eq! (+ LZRO (ntrv-row) (rzro-row)) MICRO))

(defconstraint left-zero-decrements ()
  (if-eq LZRO 1 (did-dec! TOTLZ 1)))

(defconstraint nt-decrements ()
  (if-eq (ntrv-row) 1 (did-dec! TOTNT 1)))

(defconstraint right-zero-decrements ()
  (if-eq (rzro-row) 1 (did-dec! TOTRZ 1)))

(defconstraint is-nt-only-row (:guard NT_ONLY)
  (begin (vanishes! (prev (ntrv-row)))
         (vanishes! TOTNT)))

(defconstraint is-nt-first-row (:guard NT_FIRST)
  (begin (vanishes! (prev (ntrv-row)))
         (eq! (~ TOTNT) 1)))

(defconstraint is-nt-middle-row (:guard NT_MDDL)
  (begin (eq! (prev (ntrv-row)) 1)
         (eq! (~ TOTNT) 1)))

(defconstraint is-nt-last-row (:guard NT_LAST)
  (begin (eq! (prev (ntrv-row)) 1)
         (vanishes! TOTNT)))

(defconstraint is-rz-only-row (:guard RZ_ONLY)
  (begin (vanishes! (prev (rzro-row)))
         (vanishes! TOTRZ)))

(defconstraint is-rz-first-row (:guard RZ_FIRST)
  (begin (vanishes! (prev (rzro-row)))
         (eq! (~ TOTRZ) 1)))

(defconstraint is-rz-middle-row (:guard RZ_MDDL)
  (begin (eq! (prev (rzro-row)) 1)
         (eq! (~ TOTRZ) 1)))

(defconstraint is-rz-last-row (:guard RZ_LAST)
  (begin (eq! (prev (rzro-row)) 1)
         (vanishes! TOTRZ)))

;;
;; Setting nb of preprocessing rows
;;
(defconstraint set-prprc-ct-init (:guard MACRO)
  (will-eq! prprc/CT
            (+ (* NB_PP_ROWS_MLOAD IS_MLOAD)
               (* NB_PP_ROWS_MSTORE IS_MSTORE)
               (* NB_PP_ROWS_MSTORE8 IS_MSTORE8)
               (* NB_PP_ROWS_INVALID_CODE_PREFIX IS_INVALID_CODE_PREFIX)
               (* NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION IS_RIGHT_PADDED_WORD_EXTRACTION)
               (* NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING IS_RAM_TO_EXO_WITH_PADDING)
               (* NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS IS_EXO_TO_RAM_TRANSPLANTS)
               (* NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING IS_RAM_TO_RAM_SANS_PADDING)
               (* NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA)
               (* NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)
               (* NB_PP_ROWS_MODEXP_ZERO IS_MODEXP_ZERO)
               (* NB_PP_ROWS_MODEXP_DATA IS_MODEXP_DATA)
               (* NB_PP_ROWS_BLAKE IS_BLAKE))))

;;
;; Utilities
;;
(defun (callToEuc dividend divisor)
  (begin (eq! prprc/EUC_FLAG 1)
         (eq! prprc/EUC_A dividend)
         (eq! prprc/EUC_B divisor)))

(defun (callToLt arg1hi arg1lo arg2lo)
  (begin (eq! prprc/WCP_FLAG 1)
         (eq! prprc/WCP_INST LT)
         (eq! prprc/WCP_ARG_1_HI arg1hi)
         (eq! prprc/WCP_ARG_1_LO arg1lo)
         (eq! prprc/WCP_ARG_2_LO arg2lo)))

(defun (callToEq arg1hi arg1lo arg2lo)
  (begin (eq! prprc/WCP_FLAG 1)
         (eq! prprc/WCP_INST EQ_)
         (eq! prprc/WCP_ARG_1_HI arg1hi)
         (eq! prprc/WCP_ARG_1_LO arg1lo)
         (eq! prprc/WCP_ARG_2_LO arg2lo)))

(defun (callToIszero arg1hi arg1lo)
  (begin (eq! prprc/WCP_FLAG 1)
         (eq! prprc/WCP_INST ISZERO)
         (eq! prprc/WCP_ARG_1_HI arg1hi)
         (eq! prprc/WCP_ARG_1_LO arg1lo)
         (debug (vanishes! prprc/WCP_ARG_2_LO))))

(defun (stdProgression C)
  (eq! C
       (* (prev MICRO)
          (+ (prev C) 1))))

;;;;;;;;;;;;;;;;;;;;;;;
;;                   ;;
;;  MMU Instruction  ;;
;;                   ;;
;;;;;;;;;;;;;;;;;;;;;;;
;;
;; MLOAD
;;
(defun (mload-aligned)
  (force-bool (next prprc/WCP_RES)))

(defun (mload-slo)
  (next prprc/EUC_QUOT))

(defun (mload-sbo)
  (next prprc/EUC_REM))

(defconstraint mload-pre-processing (:guard (* MACRO IS_MLOAD))
  (begin  ;; setting tot nb of mmio inst
         (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_MLOAD)
         (vanishes! TOTRZ)
         ;; setting prprc row n°1
         (next (callToEuc macro/SRC_OFFSET_LO LLARGE))
         (next (callToIszero 0 (mload-sbo)))
         ;;setting mmio constant values
         (eq! (shift micro/CN_S NB_PP_ROWS_MLOAD_PO) macro/SRC_ID)
         (vanishes! (shift micro/EXO_SUM NB_PP_ROWS_MLOAD_PO))
         ;; setting first mmio inst
         (if-zero (mload-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PO) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PO) MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
         (eq! (shift micro/SLO NB_PP_ROWS_MLOAD_PO) (mload-slo))
         (eq! (shift micro/SBO NB_PP_ROWS_MLOAD_PO) (mload-sbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MLOAD_PO) macro/LIMB_1)
         ;; setting second mmio inst
         (if-zero (mload-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PT) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PT) MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
         (eq! (shift micro/SLO NB_PP_ROWS_MLOAD_PT) (+ (mload-slo) 1))
         (eq! (shift micro/SBO NB_PP_ROWS_MLOAD_PT) (mload-sbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MLOAD_PT) macro/LIMB_2)))

;;
;; MSTORE
;;
(defun (mstore-aligned)
  (force-bool (next prprc/WCP_RES)))

(defun (mstore-tlo)
  (next prprc/EUC_QUOT))

(defun (mstore-tbo)
  (next prprc/EUC_REM))

(defconstraint mstore-pre-processing (:guard (* MACRO IS_MSTORE))
  (begin  ;; setting tot nb of mmio inst
         (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_MSTORE)
         (vanishes! TOTRZ)
         ;; setting prprc row n°1
         (next (callToEuc macro/TGT_OFFSET_LO LLARGE))
         (next (callToIszero 0 (mstore-tbo)))
         ;;setting mmio constant values
         (eq! (shift micro/CN_T NB_PP_ROWS_MSTORE_PO) macro/TGT_ID)
         ;; setting first mmio inst
         (if-zero (mstore-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PO) MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PO) MMIO_INST_LIMB_TO_RAM_TRANSPLANT))
         (eq! (shift micro/TLO NB_PP_ROWS_MSTORE_PO) (mstore-tlo))
         (eq! (shift micro/TBO NB_PP_ROWS_MSTORE_PO) (mstore-tbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MSTORE_PO) macro/LIMB_1)
         ;; setting second mmio inst
         (if-zero (mstore-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PT) MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PT) MMIO_INST_LIMB_TO_RAM_TRANSPLANT))
         (eq! (shift micro/TLO NB_PP_ROWS_MSTORE_PT) (+ (mstore-tlo) 1))
         (eq! (shift micro/TBO NB_PP_ROWS_MSTORE_PT) (mstore-tbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MSTORE_PT) macro/LIMB_2)))

;;
;; MSTORE8
;;
(defun (mstore8-tlo)
  (next prprc/EUC_QUOT))

(defun (mstore8-tbo)
  (next prprc/EUC_REM))

(defconstraint mstore8-pre-processing (:guard (* MACRO IS_MSTORE8))
  (begin  ;; setting tot nb of mmio inst
         (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_MSTORE_EIGHT)
         (vanishes! TOTRZ)
         ;; setting prprc row n°1
         (next (callToEuc macro/TGT_OFFSET_LO LLARGE))
         ;; setting first mmio inst
         (eq! (shift micro/INST NB_PP_ROWS_MSTORE8_PO) MMIO_INST_LIMB_TO_RAM_ONE_TARGET)
         (eq! (shift micro/TLO NB_PP_ROWS_MSTORE8_PO) (mstore8-tlo))
         (eq! (shift micro/TBO NB_PP_ROWS_MSTORE8_PO) (mstore8-tbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MSTORE8_PO) macro/LIMB_2)
         (eq! (shift micro/CN_T NB_PP_ROWS_MSTORE8_PO) macro/TGT_ID)))

;;
;; INVALID CODE PREFIX
;;
(defun (invalid-code-prefix-slo)
  (next prprc/EUC_QUOT))

(defun (invalid-code-prefix-sbo)
  (next prprc/EUC_REM))

(defconstraint invalid-code-prefix-pre-processing (:guard (* MACRO IS_INVALID_CODE_PREFIX))
  (begin  ;; setting tot nb of mmio inst
         (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_INVALID_CODE_PREFIX)
         (vanishes! TOTRZ)
         ;; setting prprc row n°1
         (next (callToEuc macro/SRC_OFFSET_LO LLARGE))
         (next (callToEq 0 (shift micro/LIMB NB_PP_ROWS_INVALID_CODE_PREFIX_PO) INVALID_CODE_PREFIX_VALUE))
         ;; setting the success bit
         (eq! macro/SUCCESS_BIT
              (- 1 (next prprc/WCP_RES)))
         ;; setting first mmio inst
         (eq! (shift micro/INST NB_PP_ROWS_INVALID_CODE_PREFIX_PO) MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
         (eq! (shift micro/SIZE NB_PP_ROWS_INVALID_CODE_PREFIX_PO) 1)
         (eq! (shift micro/SLO NB_PP_ROWS_INVALID_CODE_PREFIX_PO) (invalid-code-prefix-slo))
         (eq! (shift micro/SBO NB_PP_ROWS_INVALID_CODE_PREFIX_PO) (invalid-code-prefix-sbo))
         (eq! (shift micro/TBO NB_PP_ROWS_INVALID_CODE_PREFIX_PO) LLARGEMO)
         (vanishes! (shift micro/EXO_SUM NB_PP_ROWS_INVALID_CODE_PREFIX_PO))
         (eq! (shift micro/CN_S NB_PP_ROWS_INVALID_CODE_PREFIX_PO) macro/SRC_ID)))

;;
;; RIGHT PADDED WORD EXTRACTION
;;
(defun (right-pad-word-extract-second-limb-padded)
  (- 1 (next prprc/WCP_RES)))

(defun (right-pad-word-extract-extract-size)
  (+ (* (right-pad-word-extract-second-limb-padded) (- macro/REF_SIZE macro/SRC_OFFSET_LO))
     (* (- 1 (right-pad-word-extract-second-limb-padded)) WORD_SIZE)))

(defun (right-pad-word-extract-first-limb-padded)
  (shift prprc/WCP_RES 2))

(defun (right-pad-word-extract-second-limb-byte-size)
  (+ (* (- 1 (right-pad-word-extract-second-limb-padded)) LLARGE)
     (* (right-pad-word-extract-second-limb-padded)
        (+ (* (- 1 (right-pad-word-extract-first-limb-padded))
              (- (right-pad-word-extract-extract-size) LLARGE))
           (* (right-pad-word-extract-first-limb-padded) 0)))))

(defun (right-pad-word-extract-first-limb-byte-size)
  (+ (* (- 1 (right-pad-word-extract-first-limb-padded)) LLARGE)
     (* (right-pad-word-extract-first-limb-padded) (right-pad-word-extract-extract-size))))

(defun (right-pad-word-extract-first-limb-is-full)
  (force-bool (shift prprc/EUC_QUOT 2)))

(defun (right-pad-word-extract-aligned)
  (force-bool (next prprc/WCP_RES)))

(defun (right-pad-word-extract-slo)
  (shift prprc/EUC_QUOT 3))

(defun (right-pad-word-extract-sbo)
  (shift prprc/EUC_REM 3))

(defun (right-pad-word-extract-first-limb-single-source)
  (force-bool (shift prprc/WCP_RES 3)))

(defun (right-pad-word-extract-second-limb-single-source)
  (force-bool (shift prprc/WCP_RES 4)))

(defun (right-pad-word-extract-second-limb-void)
  (force-bool (shift prprc/WCP_RES 5)))

(defconstraint right-pad-word-extract-pre-processing (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
  (begin  ;; setting tot nb of mmio inst
         (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_RIGHT_PADDED_WORD_EXTRACTION)
         (vanishes! TOTRZ)
         ;; setting prprc row n°1
         (next (callToLt 0 (+ macro/SRC_OFFSET_LO WORD_SIZE) macro/REF_SIZE))
         ;; setting prprc row n°2
         (shift (callToLt 0 (right-pad-word-extract-extract-size) LLARGE) 2)
         (shift (callToEuc (right-pad-word-extract-first-limb-byte-size) LLARGE) 2)
         ;; setting prprc row n°3
         (shift (callToEuc (+ macro/SRC_OFFSET_LO macro/REF_OFFSET) LLARGE)
                3)
         (shift (callToLt 0
                          (+ (right-pad-word-extract-sbo) (right-pad-word-extract-first-limb-byte-size))
                          LLARGEPO)
                3)
         ;; setting prprc row n°4
         (shift (callToLt 0
                          (+ (right-pad-word-extract-sbo) (right-pad-word-extract-second-limb-byte-size))
                          LLARGEPO)
                4)
         ;; setting prprc row n°5
         (shift (callToIszero 0 (right-pad-word-extract-second-limb-byte-size)) 5)
         ;;setting mmio constant values
         (eq! (shift micro/CN_S NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) macro/SRC_ID)
         (vanishes! (shift micro/EXO_SUM NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO))
         ;; setting first mmio inst
         (if-zero (right-pad-word-extract-first-limb-single-source)
                  (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO)
                       MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (if-zero (right-pad-word-extract-first-limb-is-full)
                           (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO)
                                MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
                           (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO)
                                MMIO_INST_RAM_TO_LIMB_TRANSPLANT)))
         (eq! (shift micro/SIZE NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO)
              (right-pad-word-extract-first-limb-byte-size))
         (eq! (shift micro/SLO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) (right-pad-word-extract-slo))
         (eq! (shift micro/SBO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) (right-pad-word-extract-sbo))
         (vanishes! (shift micro/TBO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO))
         (eq! (shift micro/LIMB NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) macro/LIMB_1)
         ;; setting second mmio inst
         (if-zero (right-pad-word-extract-second-limb-void)
                  (if-zero (right-pad-word-extract-second-limb-single-source)
                           (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                           (if-zero (right-pad-word-extract-second-limb-padded)
                                    (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                         MMIO_INST_RAM_TO_LIMB_TRANSPLANT)
                                    (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                         MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)))
                  (begin (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                              MMIO_INST_LIMB_VANISHES)
                         (vanishes! macro/LIMB_2)))
         (eq! (shift micro/SIZE NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
              (right-pad-word-extract-second-limb-byte-size))
         (eq! (shift micro/SLO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
              (+ (right-pad-word-extract-slo) 1))
         (eq! (shift micro/SBO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) (right-pad-word-extract-sbo))
         (vanishes! (shift micro/TBO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT))
         (eq! (shift micro/LIMB NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) macro/LIMB_1)))

;;
;; RAM TO EXO WITH PADDING
;;
(defun (ram-exo-wpad-aligned)
  (force-bool (next prprc/WCP_RES)))

(defun (ram-exo-wpad-initial-slo)
  (next prprc/EUC_QUOT))

(defun (ram-exo-wpad-initial-sbo)
  (next prprc/EUC_REM))

(defun (ram-exo-wpad-has-right-padding)
  (force-bool (shift prprc/WCP_RES 2)))

(defun (ram-exo-wpad-padding-size)
  (* (ram-exo-wpad-has-right-padding) (- macro/REF_SIZE macro/SIZE)))

(defun (ram-exo-wpad-extraction-size)
  (+ (* (ram-exo-wpad-has-right-padding) macro/SIZE)
     (* (- 1 (ram-exo-wpad-has-right-padding)) macro/REF_SIZE)))

(defun (ram-exo-wpad-last-limb-is-full)
  (force-bool (shift prprc/WCP_RES 3)))

(defun (ram-exo-wpad-last-limb-byte-size)
  (+ (* (ram-exo-wpad-last-limb-is-full) LLARGE)
     (* (- 1 (ram-exo-wpad-last-limb-is-full)) (shift prprc/EUC_REM 3))))

(defun (ram-exo-wpad-last-limb-single-source)
  (force-bool (shift prprc/WCP_RES 4)))

(defconstraint ram-to-exo-with-padding-preprocessing (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
  (begin  ;; setting nb of rows
         (vanishes! TOTLZ)
         ;; setting bins and out
         (eq! [BIN 1] (ram-exo-wpad-aligned))
         (eq! [OUT 1] (ram-exo-wpad-last-limb-byte-size))
         (eq! [BIN 2] (ram-exo-wpad-last-limb-single-source))
         (eq! [BIN 3] (ram-exo-wpad-last-limb-is-full))
         ;; setting prprc row n°1
         (next (callToEuc macro/SRC_OFFSET_LO LLARGE))
         (next (callToIszero 0 (ram-exo-wpad-initial-sbo)))
         ;; setting prprc row n°2
         (shift (callToLt 0 macro/SIZE macro/REF_SIZE) 2)
         (shift (callToEuc (ram-exo-wpad-padding-size) LLARGE) 2)
         ;; setting prprc row n°3
         (shift (callToIszero 0 prprc/EUC_REM) 3)
         (shift (callToEuc (ram-exo-wpad-extraction-size) LLARGE) 3)
         ;; setting prprc row n°4
         (shift (callToLt 0
                          (+ (ram-exo-wpad-initial-sbo) (- (ram-exo-wpad-last-limb-byte-size) 1))
                          LLARGE)
                4)))

;; TODO finish microinstruction writing


