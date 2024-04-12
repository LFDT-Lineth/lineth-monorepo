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
               (any! (remained-constant! X) (did-dec! X 1))))

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
  (force-bool (+ IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)))

(defun (inst-flag-sum)
  (force-bool (+ IS_MLOAD
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
                 IS_BLAKE)))

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
  (force-bool (+ NT_ONLY NT_FIRST NT_MDDL NT_LAST)))

(defun (rzro-row)
  (force-bool (+ RZ_ONLY RZ_FIRST RZ_MDDL RZ_LAST)))

(defun (zero-row)
  (force-bool (+ LZRO (rzro-row))))

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
  (eq! (next prprc/CT)
       (+ (* (- NB_PP_ROWS_MLOAD 1) IS_MLOAD)
          (* (- NB_PP_ROWS_MSTORE 1) IS_MSTORE)
          (* (- NB_PP_ROWS_MSTORE8 1) IS_MSTORE8)
          (* (- NB_PP_ROWS_INVALID_CODE_PREFIX 1) IS_INVALID_CODE_PREFIX)
          (* (- NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION 1) IS_RIGHT_PADDED_WORD_EXTRACTION)
          (* (- NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING 1) IS_RAM_TO_EXO_WITH_PADDING)
          (* (- NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS 1) IS_EXO_TO_RAM_TRANSPLANTS)
          (* (- NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING 1) IS_RAM_TO_RAM_SANS_PADDING)
          (* (- NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA 1) IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA)
          (* (- NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING 1) IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)
          (* (- NB_PP_ROWS_MODEXP_ZERO 1) IS_MODEXP_ZERO)
          (* (- NB_PP_ROWS_MODEXP_DATA 1) IS_MODEXP_DATA)
          (* (- NB_PP_ROWS_BLAKE 1) IS_BLAKE))))

;;
;; Utilities
;;
(defun (callToEuc row dividend divisor)
  (begin (eq! (shift prprc/EUC_FLAG row) 1)
         (eq! (shift prprc/EUC_A row) dividend)
         (eq! (shift prprc/EUC_B row) divisor)))

(defun (callToLt row arg1hi arg1lo arg2lo)
  (begin (eq! (shift prprc/WCP_FLAG row) 1)
         (eq! (shift prprc/WCP_INST row) EVM_INST_LT)
         (eq! (shift prprc/WCP_ARG_1_HI row) arg1hi)
         (eq! (shift prprc/WCP_ARG_1_LO row) arg1lo)
         (eq! (shift prprc/WCP_ARG_2_LO row) arg2lo)))

(defun (callToEq row arg1hi arg1lo arg2lo)
  (begin (eq! (shift prprc/WCP_FLAG row) 1)
         (eq! (shift prprc/WCP_INST row) EVM_INST_EQ)
         (eq! (shift prprc/WCP_ARG_1_HI row) arg1hi)
         (eq! (shift prprc/WCP_ARG_1_LO row) arg1lo)
         (eq! (shift prprc/WCP_ARG_2_LO row) arg2lo)))

(defun (callToIszero row arg1hi arg1lo)
  (begin (eq! (shift prprc/WCP_FLAG row) 1)
         (eq! (shift prprc/WCP_INST row) EVM_INST_ISZERO)
         (eq! (shift prprc/WCP_ARG_1_HI row) arg1hi)
         (eq! (shift prprc/WCP_ARG_1_LO row) arg1lo)
         (debug (vanishes! (shift prprc/WCP_ARG_2_LO row)))))

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
  (next prprc/WCP_RES))

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
         (callToEuc 1 macro/SRC_OFFSET_LO LLARGE)
         (callToIszero 1 0 (mload-sbo))
         ;;setting mmio constant values
         (eq! (shift micro/CN_S NB_PP_ROWS_MLOAD_PO) macro/SRC_ID)
         (vanishes! (shift micro/EXO_SUM NB_PP_ROWS_MLOAD_PO))
         ;; setting first mmio inst
         (if-zero (mload-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PO) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PO) MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
         (eq! (shift micro/SIZE NB_PP_ROWS_MLOAD_PO) LLARGE)
         (eq! (shift micro/SLO NB_PP_ROWS_MLOAD_PO) (mload-slo))
         (eq! (shift micro/SBO NB_PP_ROWS_MLOAD_PO) (mload-sbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MLOAD_PO) macro/LIMB_1)
         ;; setting second mmio inst
         (if-zero (mload-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PT) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! (shift micro/INST NB_PP_ROWS_MLOAD_PT) MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
         (eq! (shift micro/SIZE NB_PP_ROWS_MLOAD_PT) LLARGE)
         (eq! (shift micro/SLO NB_PP_ROWS_MLOAD_PT) (+ (mload-slo) 1))
         (eq! (shift micro/SBO NB_PP_ROWS_MLOAD_PT) (mload-sbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MLOAD_PT) macro/LIMB_2)))

;;
;; MSTORE
;;
(defun (mstore-aligned)
  (next prprc/WCP_RES))

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
         (callToEuc 1 macro/TGT_OFFSET_LO LLARGE)
         (callToIszero 1 0 (mstore-tbo))
         ;;setting mmio constant values
         (eq! (shift micro/CN_T NB_PP_ROWS_MSTORE_PO) macro/TGT_ID)
         ;; setting first mmio inst
         (if-zero (mstore-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PO) MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PO) MMIO_INST_LIMB_TO_RAM_TRANSPLANT))
         (eq! (shift micro/SIZE NB_PP_ROWS_MSTORE_PO) LLARGE)
         (eq! (shift micro/TLO NB_PP_ROWS_MSTORE_PO) (mstore-tlo))
         (eq! (shift micro/TBO NB_PP_ROWS_MSTORE_PO) (mstore-tbo))
         (eq! (shift micro/LIMB NB_PP_ROWS_MSTORE_PO) macro/LIMB_1)
         ;; setting second mmio inst
         (if-zero (mstore-aligned)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PT) MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                  (eq! (shift micro/INST NB_PP_ROWS_MSTORE_PT) MMIO_INST_LIMB_TO_RAM_TRANSPLANT))
         (eq! (shift micro/SIZE NB_PP_ROWS_MSTORE_PT) LLARGE)
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
         (callToEuc 1 macro/TGT_OFFSET_LO LLARGE)
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
         (callToEuc 1 macro/SRC_OFFSET_LO LLARGE)
         (callToEq 1 0 (shift micro/LIMB NB_PP_ROWS_INVALID_CODE_PREFIX_PO) EIP_3541_MARKER)
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
  (force-bool (- 1 (next prprc/WCP_RES))))

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
  (next prprc/WCP_RES))

(defun (right-pad-word-extract-slo)
  (shift prprc/EUC_QUOT 3))

(defun (right-pad-word-extract-sbo)
  (shift prprc/EUC_REM 3))

(defun (right-pad-word-extract-first-limb-single-source)
  (shift prprc/WCP_RES 3))

(defun (right-pad-word-extract-second-limb-single-source)
  (shift prprc/WCP_RES 4))

(defun (right-pad-word-extract-second-limb-void)
  (shift prprc/WCP_RES 5))

(defconstraint right-pad-word-extract-pre-processing (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
  (begin  ;; setting tot nb of mmio inst
         (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_RIGHT_PADDED_WORD_EXTRACTION)
         (vanishes! TOTRZ)
         ;; setting prprc row n°1
         (callToLt 1 0 (+ macro/SRC_OFFSET_LO WORD_SIZE) macro/REF_SIZE)
         ;; setting prprc row n°2
         (callToLt 2 0 (right-pad-word-extract-extract-size) LLARGE)
         (callToEuc 2 (right-pad-word-extract-first-limb-byte-size) LLARGE)
         ;; setting prprc row n°3
         (callToEuc 3 (+ macro/SRC_OFFSET_LO macro/REF_OFFSET) LLARGE)
         (callToLt 3
                   0
                   (+ (right-pad-word-extract-sbo) (right-pad-word-extract-first-limb-byte-size))
                   LLARGEPO)
         ;; setting prprc row n°4
         (callToLt 4
                   0
                   (+ (right-pad-word-extract-sbo) (right-pad-word-extract-second-limb-byte-size))
                   LLARGEPO)
         ;; setting prprc row n°5
         (callToIszero 5 0 (right-pad-word-extract-second-limb-byte-size))
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
         (if-eq-else (right-pad-word-extract-second-limb-void) 1
                     (begin (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                 MMIO_INST_LIMB_VANISHES)
                            (vanishes! macro/LIMB_2))
                     (if-eq-else (right-pad-word-extract-second-limb-single-source) 1
                                 (if-zero (right-pad-word-extract-second-limb-padded)
                                          (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                               MMIO_INST_RAM_TO_LIMB_TRANSPLANT)
                                          (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                               MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                                 (eq! (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
                                      MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)))
         (eq! (shift micro/SIZE NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
              (right-pad-word-extract-second-limb-byte-size))
         (eq! (shift micro/SLO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)
              (+ (right-pad-word-extract-slo) 1))
         (eq! (shift micro/SBO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) (right-pad-word-extract-sbo))
         (vanishes! (shift micro/TBO NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT))
         (eq! (shift micro/LIMB NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) macro/LIMB_2)))

;;
;; RAM TO EXO WITH PADDING
;;
(defun (ram-exo-wpad-aligned)
  (next prprc/WCP_RES))

(defun (ram-exo-wpad-initial-slo)
  (next prprc/EUC_QUOT))

(defun (ram-exo-wpad-initial-sbo)
  (next prprc/EUC_REM))

(defun (ram-exo-wpad-has-right-padding)
  (shift prprc/WCP_RES 2))

(defun (ram-exo-wpad-padding-size)
  (* (ram-exo-wpad-has-right-padding) (- macro/REF_SIZE macro/SIZE)))

(defun (ram-exo-wpad-extraction-size)
  (+ (* (ram-exo-wpad-has-right-padding) macro/SIZE)
     (* (- 1 (ram-exo-wpad-has-right-padding)) macro/REF_SIZE)))

(defun (ram-exo-wpad-last-limb-is-full)
  (shift prprc/WCP_RES 3))

(defun (ram-exo-wpad-last-limb-byte-size)
  (+ (* (ram-exo-wpad-last-limb-is-full) LLARGE)
     (* (- 1 (ram-exo-wpad-last-limb-is-full)) (shift prprc/EUC_REM 3))))

(defun (ram-exo-wpad-last-limb-single-source)
  (shift prprc/WCP_RES 4))

(defconstraint ram-to-exo-with-padding-preprocessing (:guard (* MACRO IS_RAM_TO_EXO_WITH_PADDING))
  (begin  ;; setting nb of rows
         (vanishes! TOTLZ)
         ;; setting bins and out
         (eq! [BIN 1] (ram-exo-wpad-aligned))
         (eq! [OUT 1] (ram-exo-wpad-last-limb-byte-size))
         (eq! [BIN 2] (ram-exo-wpad-last-limb-single-source))
         (eq! [BIN 3] (ram-exo-wpad-last-limb-is-full))
         ;; setting prprc row n°1
         (callToEuc 1 macro/SRC_OFFSET_LO LLARGE)
         (callToIszero 1 0 (ram-exo-wpad-initial-sbo))
         ;; setting prprc row n°2
         (callToLt 2 0 macro/SIZE macro/REF_SIZE)
         (callToEuc 2 (ram-exo-wpad-padding-size) LLARGE)
         ;; setting prprc row n°3
         (callToIszero 3 0 (shift prprc/EUC_REM 3))
         (callToEuc 3 (ram-exo-wpad-extraction-size) LLARGE)
         ;; setting prprc row n°4
         (callToLt 4
                   0
                   (+ (ram-exo-wpad-initial-sbo) (- (ram-exo-wpad-last-limb-byte-size) 1))
                   LLARGE)
         ;; setting mmio constant values
         (eq! (shift micro/CN_S NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/SRC_ID)
         (eq! (shift micro/SUCCESS_BIT NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/SUCCESS_BIT)
         (eq! (shift micro/EXO_SUM NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/EXO_SUM)
         (eq! (shift micro/PHASE NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/PHASE)
         (eq! (shift micro/EXO_ID NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/TGT_ID)
         (eq! (shift micro/KEC_ID NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/AUX_ID)
         (eq! (shift micro/TOTAL_SIZE NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO) macro/REF_SIZE)))

(defconstraint ram-to-exo-with-padding-std-progression (:guard (* IS_RAM_TO_EXO_WITH_PADDING MICRO))
  (begin (stdProgression micro/TLO)
         (vanishes! micro/TBO)))

(defconstraint ram-to-exo-with-padding-first-row (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_FIRST))
  (begin (if-zero [BIN 1]
                  (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
         (eq! micro/SIZE LLARGE)
         (eq! micro/SLO
              (shift (ram-exo-wpad-initial-slo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))
         (eq! micro/SBO
              (shift (ram-exo-wpad-initial-sbo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))))

(defconstraint ram-to-exo-with-padding-middle (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_MDDL))
  (begin (if-zero [BIN 1]
                  (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
         (eq! micro/SIZE LLARGE)
         (did-inc! micro/SLO 1)
         (remained-constant! micro/SBO)))

(defconstraint ram-to-exo-with-padding-last (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_LAST))
  (begin (did-inc! micro/SLO 1)
         (remained-constant! micro/SBO)))

(defconstraint ram-to-exo-with-padding-only (:guard (* IS_RAM_TO_EXO_WITH_PADDING NT_ONLY))
  (begin (eq! micro/SLO
              (shift (ram-exo-wpad-initial-slo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))
         (eq! micro/SBO
              (shift (ram-exo-wpad-initial-sbo) (- 0 NB_PP_ROWS_RAM_TO_EXO_WITH_PADDING_PO)))))

(defconstraint ram-to-exo-with-padding-last-or-only-common (:guard (* IS_RAM_TO_EXO_WITH_PADDING
      (force-bool (+ NT_LAST NT_ONLY))))
  (begin (eq! micro/SIZE [OUT 1])
         (if-zero [BIN 2]
                  (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (if-zero [BIN 3]
                           (eq! micro/INST MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
                           (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT)))))

(defconstraint ram-to-exo-with-padding-right-zeroes (:guard (* IS_RAM_TO_EXO_WITH_PADDING (rzro-row)))
  (eq! micro/INST MMIO_INST_LIMB_VANISHES))

;;
;; EXO TO RAM TRANSPLANT
;;
(defconstraint exo-to-ram-preprocessing (:guard (* IS_EXO_TO_RAM_TRANSPLANTS MACRO))
  (begin  ;; setting prprc row n°1
         (callToEuc 1 macro/SIZE LLARGE)
         ;; setting nb of rows
         (vanishes! TOTLZ)
         (eq! TOTNT (next prprc/EUC_CEIL))
         (vanishes! TOTRZ)
         ;; setting mmio constant values
         (eq! (shift micro/CN_T NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO) macro/TGT_ID)
         (eq! (shift micro/EXO_SUM NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO) macro/EXO_SUM)
         (eq! (shift micro/PHASE NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO) macro/PHASE)
         (eq! (shift micro/EXO_ID NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO) macro/SRC_ID)))

(defconstraint exo-to-ram-micro-inst-writting (:guard (* IS_EXO_TO_RAM_TRANSPLANTS MICRO))
  (begin (stdProgression micro/SLO)
         (stdProgression micro/TLO)
         (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TRANSPLANT)))

;;
;; RAM TO RAM SANS PADDING
;;
(defun (ram-to-ram-sans-pad-last-limb-byte-size)
  [OUT 1])

(defun (ram-to-ram-sans-pad-middle-sbo)
  [OUT 2])

(defun (ram-to-ram-sans-pad-aligned)
  [BIN 1])

(defun (ram-to-ram-sans-pad-last-limb-single-source)
  [BIN 2])

(defun (ram-to-ram-sans-pad-initial-slo-increment)
  [BIN 3])

(defun (ram-to-ram-sans-pad-last-limb-is-fast)
  [BIN 4])

(defun (ram-to-ram-sans-pad-rdo)
  macro/SRC_OFFSET_LO)

(defun (ram-to-ram-sans-pad-rds)
  macro/SIZE)

(defun (ram-to-ram-sans-pad-rato)
  macro/REF_OFFSET)

(defun (ram-to-ram-sans-pad-ratc)
  macro/REF_SIZE)

(defun (ram-to-ram-sans-pad-initial-slo)
  (next prprc/EUC_QUOT))

(defun (ram-to-ram-sans-pad-initial-sbo)
  (next prprc/EUC_REM))

(defun (ram-to-ram-sans-pad-initial-cmp)
  (next prprc/WCP_RES))

(defun (ram-to-ram-sans-pad-initial-real-size)
  (+ (* (ram-to-ram-sans-pad-initial-cmp) (ram-to-ram-sans-pad-ratc))
     (* (- 1 (ram-to-ram-sans-pad-initial-cmp)) (ram-to-ram-sans-pad-rds))))

(defun (ram-to-ram-sans-pad-initial-tlo)
  (shift prprc/EUC_QUOT 2))

(defun (ram-to-ram-sans-pad-initial-tbo)
  (shift prprc/EUC_REM 2))

(defun (ram-to-ram-sans-pad-final-tlo)
  (shift prprc/EUC_QUOT 3))

(defun (ram-to-ram-sans-pad-totnt-is-one)
  (shift prprc/WCP_RES 3))

(defun (ram-to-ram-sans-pad-first-limb-byte-size)
  (+ (* (ram-to-ram-sans-pad-totnt-is-one) (ram-to-ram-sans-pad-initial-real-size))
     (* (- 1 (ram-to-ram-sans-pad-totnt-is-one)) (- LLARGE (ram-to-ram-sans-pad-initial-tbo)))))

(defun (ram-to-ram-sans-pad-first-limb-single-source)
  (shift prprc/WCP_RES 4))

(defun (ram-to-ram-sans-pad-init-tbo-is-zero)
  (shift prprc/WCP_RES 5))

(defun (ram-to-ram-sans-pad-last-limb-is-full)
  (force-bool (shift prprc/EUC_QUOT 5)))

(defun (ram-to-ram-sans-pad-first-limb-is-fast)
  (force-bool (* (ram-to-ram-sans-pad-aligned) (ram-to-ram-sans-pad-init-tbo-is-zero))))

(defconstraint ram-to-ram-sans-pad-preprocessing (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
  (begin  ;; set nb of rows
         (vanishes! TOTLZ)
         (vanishes! TOTRZ)
         ;; preprocessing row n°1
         (callToEuc 1 (ram-to-ram-sans-pad-rdo) LLARGE)
         (callToLt 1 0 (ram-to-ram-sans-pad-ratc) (ram-to-ram-sans-pad-rds))
         ;; preprocessing row n°2
         (callToEuc 2 (ram-to-ram-sans-pad-rato) LLARGE)
         (callToEq 2 0 (ram-to-ram-sans-pad-initial-sbo) (ram-to-ram-sans-pad-initial-tbo))
         (eq! (ram-to-ram-sans-pad-aligned) (shift prprc/WCP_RES 2))
         ;; preprocessing row n°3
         (callToEuc 3
                    (+ (ram-to-ram-sans-pad-rato) (- (ram-to-ram-sans-pad-initial-real-size) 1))
                    LLARGE)
         (callToEq 3 0 TOTNT 1)
         (eq! TOTNT
              (+ (- (ram-to-ram-sans-pad-final-tlo) (ram-to-ram-sans-pad-initial-tlo)) 1))
         (if-zero (ram-to-ram-sans-pad-totnt-is-one)
                  (eq! (ram-to-ram-sans-pad-last-limb-byte-size)
                       (+ 1 (shift prprc/EUC_REM 3)))
                  (eq! (ram-to-ram-sans-pad-last-limb-byte-size) (ram-to-ram-sans-pad-initial-real-size)))
         ;; preprocessing row n°4
         (callToLt 4
                   0
                   (+ (ram-to-ram-sans-pad-initial-sbo) (- (ram-to-ram-sans-pad-first-limb-byte-size) 1))
                   LLARGE)
         (callToEuc 4
                    (+ (ram-to-ram-sans-pad-middle-sbo) (- (ram-to-ram-sans-pad-last-limb-byte-size) 1))
                    LLARGE)
         (if-zero (ram-to-ram-sans-pad-aligned)
                  (debug (vanishes! (ram-to-ram-sans-pad-middle-sbo)))
                  (if-zero (ram-to-ram-sans-pad-first-limb-single-source)
                           (eq! (ram-to-ram-sans-pad-middle-sbo)
                                (- (+ (ram-to-ram-sans-pad-initial-sbo)
                                      (ram-to-ram-sans-pad-first-limb-byte-size))
                                   LLARGE))
                           (eq! (ram-to-ram-sans-pad-middle-sbo)
                                (+ (ram-to-ram-sans-pad-initial-sbo)
                                   (ram-to-ram-sans-pad-first-limb-byte-size)))))
         (if-zero (ram-to-ram-sans-pad-totnt-is-one)
                  (eq! (ram-to-ram-sans-pad-last-limb-single-source)
                       (force-bool (- 1 (shift prprc/EUC_QUOT 4))))
                  (eq! (ram-to-ram-sans-pad-last-limb-single-source)
                       (eq! (ram-to-ram-sans-pad-last-limb-single-source)
                            (ram-to-ram-sans-pad-first-limb-single-source))))
         (if-zero (ram-to-ram-sans-pad-aligned)
                  (eq! (ram-to-ram-sans-pad-initial-slo-increment) 1)
                  (eq! (ram-to-ram-sans-pad-initial-slo-increment)
                       (- 1 (ram-to-ram-sans-pad-first-limb-single-source))))
         ;; preprocessing row n°5
         (callToIszero 5 0 (ram-to-ram-sans-pad-initial-tbo))
         (callToEuc 5 (ram-to-ram-sans-pad-last-limb-byte-size) LLARGE)
         (eq! (ram-to-ram-sans-pad-last-limb-is-fast)
              (* (ram-to-ram-sans-pad-aligned) (ram-to-ram-sans-pad-last-limb-is-full)))))

(defconstraint ram-to-ram-sans-pad-constant-mmio-values (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
  (begin (eq! (shift micro/CN_S NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) macro/SRC_ID)
         (eq! (shift micro/CN_T NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) macro/TGT_ID)))

(defconstraint ram-to-ram-sans-pad-first-mmio-values (:guard (* MACRO IS_RAM_TO_RAM_SANS_PADDING))
  (begin (eq! (shift micro/SIZE NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO)
              (ram-to-ram-sans-pad-first-limb-byte-size))
         (eq! (shift micro/SLO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-pad-initial-slo))
         (eq! (shift micro/SBO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-pad-initial-sbo))
         (eq! (shift micro/TLO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-pad-initial-tlo))
         (eq! (shift micro/TBO NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO) (ram-to-ram-sans-pad-initial-tbo))))

(defconstraint ram-to-ram-sans-pad-mmio-inst-writting (:guard IS_RAM_TO_RAM_SANS_PADDING)
  (begin (if-eq (force-bool (+ NT_FIRST NT_MDDL)) 1
                (will-inc! micro/TLO 1))
         (if-eq NT_FIRST 1
                (eq! (next micro/SLO) (+ micro/SLO (ram-to-ram-sans-pad-initial-slo-increment))))
         (if-eq NT_MDDL 1 (will-inc! micro/SLO 1))
         (if-eq NT_ONLY 1
                (if-zero (ram-to-ram-sans-pad-last-limb-is-fast)
                         (if-zero (ram-to-ram-sans-pad-last-limb-single-source)
                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE)
                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                         (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)))
         (if-eq NT_FIRST 1
                (if-zero (shift (ram-to-ram-sans-pad-first-limb-is-fast)
                                (- 0 NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO))
                         (if-zero (shift (ram-to-ram-sans-pad-first-limb-single-source)
                                         (- 0 NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING_PO))
                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE)
                                  (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                         (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)))
         (if-eq NT_MDDL 1
                (begin (if-zero (ram-to-ram-sans-pad-aligned)
                                (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT)
                                (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE))
                       (eq! micro/SIZE LLARGE)
                       (eq! micro/SBO (ram-to-ram-sans-pad-middle-sbo))
                       (vanishes! micro/TBO)))
         (if-eq NT_LAST 1
                (begin (if-zero (ram-to-ram-sans-pad-last-limb-is-fast)
                                (if-zero (ram-to-ram-sans-pad-last-limb-single-source)
                                         (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_SOURCE)
                                         (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                                (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT))
                       (eq! micro/SIZE (ram-to-ram-sans-pad-last-limb-byte-size))
                       (eq! micro/SBO (ram-to-ram-sans-pad-middle-sbo))
                       (vanishes! micro/TBO)))))

;;
;; ANY TO RAM WITH PADDING
;;
(defun (any-to-ram-min-tgt-offset)
  macro/TGT_OFFSET_LO)

(defun (any-to-ram-max-tgt-offset)
  (+ macro/TGT_OFFSET_LO (- macro/SIZE 1)))

(defun (any-to-ram-pure-padd)
  (force-bool (- 1 (next prprc/WCP_RES))))

(defun (any-to-ram-min-tlo)
  (next prprc/EUC_QUOT))

(defun (any-to-ram-min-tbo)
  (next prprc/EUC_REM))

(defun (any-to-ram-max-src-offset-or-zero)
  (* (- 1 (any-to-ram-pure-padd)) (any-to-ram-max-tgt-offset)))

(defun (any-to-ram-mixed)
  (force-bool (* (- 1 (any-to-ram-pure-padd))
                 (- 1 (shift prprc/WCP_RES 2)))))

(defun (any-to-ram-pure-data)
  (force-bool (* (- 1 (any-to-ram-pure-padd)) (shift prprc/WCP_RES 2))))

(defun (any-to-ram-max-tlo)
  (shift prprc/EUC_QUOT 2))

(defun (any-to-ram-max-tbo)
  (shift prprc/EUC_REM 2))

(defun (any-to-ram-trsf-size)
  (+ (* (any-to-ram-mixed) (- macro/REF_SIZE macro/SRC_OFFSET_LO))
     (* (any-to-ram-pure-data) macro/REF_SIZE)))

(defun (any-to-ram-padd-size)
  (+ (* (any-to-ram-pure-padd) macro/SIZE)
     (* (any-to-ram-mixed)
        (- macro/SIZE (- macro/REF_SIZE macro/SRC_OFFSET_LO)))))

(defconstraint any-to-ram-prprc-common (:guard (* MACRO (is-any-to-ram-with-padding)))
  (begin  ;; preprocessing row n°1
         (callToLt 1 macro/SRC_OFFSET_HI macro/SRC_OFFSET_LO macro/REF_SIZE)
         (callToEuc 1 (any-to-ram-min-tgt-offset) LLARGE)
         ;; preprocessing row n°2
         (callToLt 2 0 (any-to-ram-max-src-offset-or-zero) macro/REF_SIZE)
         (callToEuc 2 (any-to-ram-max-tgt-offset) LLARGE)
         ;; justifyng the flag
         (eq! IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING (any-to-ram-pure-padd))
         (eq! IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA (+ (any-to-ram-mixed) (any-to-ram-pure-padd)))))

;;
;; PURE PADDING sub case
;;
(defun (any-to-ram-pure-padding-last-padding-is-full)
  [BIN 1])

(defun (any-to-ram-pure-padding-last-padding-size)
  [OUT 1])

(defun (any-to-ram-pure-padding-totrz-is-one)
  (shift prprc/WCP_RES 3))

(defun (any-to-ram-pure-padding-first-padding-is-full)
  (shift prprc/WCP_RES 4))

(defun (any-to-ram-pure-padding-only-padding-is-full)
  (* (any-to-ram-pure-padding-first-padding-is-full) (any-to-ram-pure-padding-last-padding-is-full)))

(defun (any-to-ram-pure-padding-first-padding-size)
  (- LLARGE (any-to-ram-min-tbo)))

(defun (any-to-ram-pure-padding-only-padding-size)
  (- LLARGE (any-to-ram-padd-size)))

(defconstraint any-to-ram-pure-padding-prprc (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
  (begin  ;; setting number of rows
         (vanishes! TOTLZ)
         (vanishes! TOTNT)
         (eq! TOTRZ
              (+ (- (any-to-ram-max-tlo) (any-to-ram-min-tlo)) 1))
         ;; preprocessing row n°3
         (callToEq 3 0 TOTRZ 1)
         ;; preprocessing row n°4
         (callToIszero 4 0 (any-to-ram-min-tbo))
         (callToEuc 4 (+ 1 (any-to-ram-max-tbo)) LLARGE)
         (eq! (any-to-ram-pure-padding-last-padding-is-full)
              (* (- 1 (any-to-ram-pure-padding-totrz-is-one)) (shift prprc/EUC_QUOT 4)))
         (eq! (any-to-ram-pure-padding-last-padding-size)
              (* (- 1 (any-to-ram-pure-padding-totrz-is-one)) (+ 1 (any-to-ram-max-tbo))))
         ;; mmio constant values
         (eq! (shift micro/CN_T NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO) macro/TGT_ID)
         ;; first and only common mmio
         (eq! (shift micro/TLO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO) (any-to-ram-min-tlo))
         (eq! (shift micro/TBO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO) (any-to-ram-min-tbo))
         (if-zero (any-to-ram-pure-padding-totrz-is-one)
                  ;; first mmio
                  (begin (if-zero (any-to-ram-pure-padding-first-padding-is-full)
                                  (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                       MMIO_INST_RAM_EXCISION)
                                  (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                       MMIO_INST_RAM_VANISHES))
                         (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                              (any-to-ram-pure-padding-first-padding-size)))
                  ;; only mmio
                  (begin (if-zero (any-to-ram-pure-padding-only-padding-is-full)
                                  (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                       MMIO_INST_RAM_EXCISION)
                                  (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                       MMIO_INST_RAM_VANISHES))
                         (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                              (any-to-ram-pure-padding-only-padding-size))))))

(defconstraint any-to-ram-pure-padding-mmio-inst (:guard IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)
  (begin (if-eq (force-bool (+ RZ_MDDL RZ_LAST)) 1
                (begin (did-inc! micro/TLO 1)
                       (vanishes! micro/TBO)))
         (if-eq RZ_MDDL 1 (eq! micro/INST MMIO_INST_RAM_VANISHES))
         (if-eq RZ_LAST 1
                (begin (if-zero (any-to-ram-pure-padding-last-padding-is-full)
                                (eq! micro/INST MMIO_INST_RAM_EXCISION)
                                (eq! micro/INST MMIO_INST_RAM_VANISHES))
                       (eq! micro/SIZE (any-to-ram-pure-padding-last-padding-size))))))

;;
;; SOME DATA CASE
;;
(defun (any-to-ram-some-data-tlo-increment-after-first-dt)
  [BIN 1])

(defun (any-to-ram-some-data-aligned)
  [BIN 2])

(defun (any-to-ram-some-data-middle-tbo)
  [OUT 1])

(defun (any-to-ram-some-data-last-dt-single-target)
  [BIN 3])

(defun (any-to-ram-some-data-last-dt-size)
  [OUT 2])

(defun (any-to-ram-some-data-tlo-increment-at-transition)
  [BIN 4])

(defun (any-to-ram-some-data-first-pbo)
  [OUT 3])

(defun (any-to-ram-some-data-first-padding-size)
  [OUT 4])

(defun (any-to-ram-some-data-last-padding-size)
  [OUT 5])

(defun (any-to-ram-some-data-data-src-is-ram)
  [BIN 5])

(defun (any-to-ram-some-data-totnt-is-one)
  (shift prprc/WCP_RES 4))

(defun (any-to-ram-some-data-only-dt-size)
  (any-to-ram-trsf-size))

(defun (any-to-ram-some-data-first-dt-size)
  (- LLARGE (any-to-ram-some-data-min-sbo)))

(defun (any-to-ram-some-data-min-src-offset)
  (+ macro/SRC_OFFSET_LO macro/REF_OFFSET))

(defun (any-to-ram-some-data-min-slo)
  (shift prprc/EUC_QUOT 5))

(defun (any-to-ram-some-data-min-sbo)
  (shift prprc/EUC_REM 5))

(defun (any-to-ram-some-data-max-src-offset)
  (+ (any-to-ram-some-data-min-src-offset) (- (any-to-ram-trsf-size) 1)))

(defun (any-to-ram-some-data-max-slo)
  (shift prprc/EUC_QUOT 6))

(defun (any-to-ram-some-data-max-sbo)
  (shift prprc/EUC_REM 6))

(defun (any-to-ram-some-data-only-dt-single-target)
  (force-bool (- 1 (shift prprc/EUC_QUOT 7))))

(defun (any-to-ram-some-data-only-dt-maxes-out-target)
  (shift prprc/WCP_RES 7))

(defun (any-to-ram-some-data-first-dt-single-target)
  (force-bool (- 1 (shift prprc/EUC_QUOT 8))))

(defun (any-to-ram-some-data-first-dt-maxes-out-target)
  (shift prprc/WCP_RES 8))

(defun (any-to-ram-some-data-last-dt-maxes-out-target)
  (shift prprc/WCP_RES 9))

(defun (any-to-ram-some-data-first-padding-offset)
  (+ (any-to-ram-min-tgt-offset) (any-to-ram-trsf-size)))

(defun (any-to-ram-some-data-first-plo)
  (shift prprc/EUC_QUOT 10))

(defun (any-to-ram-some-data-last-plo)
  (any-to-ram-max-tlo))

(defun (any-to-ram-some-data-last-pbo)
  (any-to-ram-max-tbo))

(defun (any-to-ram-some-data-totrz-is-one)
  (shift prprc/WCP_RES 10))

(defun (any-to-ram-some-data-micro-cns)
  (* (any-to-ram-some-data-data-src-is-ram) macro/SRC_ID))

(defun (any-to-ram-some-data-micro-id1)
  (* (- 1 (any-to-ram-some-data-data-src-is-ram)) macro/SRC_ID))

(defconstraint any-to-ram-some-data-preprocessing (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA))
  (begin  ;; preprocessing row n°3
         (callToIszero 3 0 macro/EXO_SUM)
         (eq! (any-to-ram-some-data-data-src-is-ram) (shift prprc/WCP_RES 3))
         ;; setting nb of rows
         (vanishes! TOTLZ)
         (eq! TOTNT
              (+ (- (any-to-ram-some-data-max-slo) (any-to-ram-some-data-min-slo)) 1))
         ;; preprocessing row n°4
         (callToEq 4 0 TOTNT 1)
         (eq! (any-to-ram-some-data-last-dt-size) (+ (any-to-ram-some-data-min-sbo) 1))
         ;; preprocessing row n°5
         (callToEuc 5 (any-to-ram-some-data-min-src-offset) LLARGE)
         (callToEq 5 0 (any-to-ram-min-tbo) (any-to-ram-some-data-min-sbo))
         (eq! (any-to-ram-some-data-aligned) (shift prprc/WCP_RES 5))
         ;; preprocessing row n°6
         (callToEuc 6 (any-to-ram-some-data-min-src-offset) LLARGE)
         ;; preprocessing row n°7
         (if-eq (any-to-ram-some-data-totnt-is-one) 1
                (begin (callToEuc 7
                                  (+ (any-to-ram-min-tbo) (- (any-to-ram-some-data-only-dt-size) 1))
                                  LLARGE)
                       (callToEq 7 0 (shift prprc/EUC_REM 7) LLARGEMO)))
         ;; preprocessing row n°8
         (if-zero (any-to-ram-some-data-totnt-is-one)
                  (begin (callToEuc 8
                                    (+ (any-to-ram-min-tbo) (- (any-to-ram-some-data-first-dt-size) 1))
                                    LLARGE)
                         (callToEq 8 0 (shift prprc/EUC_REM 8) LLARGEMO))
                  (if-zero (any-to-ram-some-data-first-dt-maxes-out-target)
                           (eq! (any-to-ram-some-data-middle-tbo)
                                (+ 1 (shift prprc/EUC_REM 8)))
                           (vanishes! (any-to-ram-some-data-middle-tbo))))
         ;; preprocessing row n°9
         (if-zero (any-to-ram-some-data-totnt-is-one)
                  (begin (callToEuc 9
                                    (+ (any-to-ram-some-data-middle-tbo)
                                       (- (any-to-ram-some-data-last-dt-size) 1))
                                    LLARGE)
                         (callToEq 9 0 (shift prprc/EUC_REM 9) LLARGEMO)
                         (eq! (any-to-ram-some-data-last-dt-single-target)
                              (- 1 (shift prprc/EUC_QUOT 9)))
                         (eq! (any-to-ram-some-data-last-dt-maxes-out-target) (shift prprc/WCP_RES 9))))
         ;; justifying tlo_increments_at_transition
         (if-eq-else (any-to-ram-some-data-totnt-is-one) 1
                     (if-zero (any-to-ram-some-data-only-dt-single-target)
                              (eq! (any-to-ram-some-data-tlo-increment-at-transition) 1)
                              (eq! (any-to-ram-some-data-tlo-increment-at-transition)
                                   (any-to-ram-some-data-only-dt-maxes-out-target)))
                     (if-zero (any-to-ram-some-data-last-dt-single-target)
                              (eq! (any-to-ram-some-data-tlo-increment-at-transition) 1)
                              (eq! (any-to-ram-some-data-tlo-increment-at-transition)
                                   (any-to-ram-some-data-last-dt-maxes-out-target))))
         ;; preprocessing row n°10
         (callToEq 10 0 TOTRZ 1)
         (callToEuc 10 (any-to-ram-some-data-first-padding-offset) LLARGE)
         (eq! (any-to-ram-some-data-first-pbo)
              (* (any-to-ram-mixed) (shift prprc/EUC_REM 10)))
         (if-eq-else (any-to-ram-some-data-totrz-is-one) 1
                     (eq! (any-to-ram-some-data-first-padding-size) (any-to-ram-padd-size))
                     (begin (eq! (any-to-ram-some-data-first-padding-size)
                                 (* (any-to-ram-mixed) (- LLARGE (any-to-ram-some-data-first-pbo))))
                            (eq! (any-to-ram-some-data-last-padding-size)
                                 (* (any-to-ram-mixed) (+ 1 (any-to-ram-some-data-first-pbo))))))
         ;; initialisation
         (eq! (shift micro/CN_S NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
              (any-to-ram-some-data-micro-cns))
         (eq! (shift micro/CN_T NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) macro/TGT_ID)
         (eq! (shift micro/EXO_SUM NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) macro/EXO_SUM)
         (eq! (shift micro/EXO_ID NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
              (any-to-ram-some-data-micro-id1))
         ;; FIRST and ONLY mmio inst shared values
         (eq! (shift micro/SLO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
              (any-to-ram-some-data-min-slo))
         (eq! (shift micro/SBO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
              (any-to-ram-some-data-min-sbo))
         (eq! (shift micro/TLO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) (any-to-ram-min-tlo))
         (eq! (shift micro/TBO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO) (any-to-ram-min-tbo))
         (if-eq-else (any-to-ram-some-data-totnt-is-one) 1
                     ;; ONLY mmio inst
                     (begin (if-eq-else (any-to-ram-some-data-data-src-is-ram) 1
                                        (if-zero (any-to-ram-some-data-only-dt-single-target)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_RAM_TO_RAM_PARTIAL))
                                        (if-zero (any-to-ram-some-data-only-dt-single-target)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_LIMB_TO_RAM_ONE_TARGET)))
                            (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                 (any-to-ram-some-data-only-dt-size)))
                     ;; FIRST mmio inst
                     (begin (if-eq-else (any-to-ram-some-data-data-src-is-ram) 1
                                        (if-zero (any-to-ram-some-data-first-dt-single-target)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_RAM_TO_RAM_PARTIAL))
                                        (if-zero (any-to-ram-some-data-first-dt-single-target)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                                 (eq! (shift micro/INST
                                                             NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                                      MMIO_INST_LIMB_TO_RAM_ONE_TARGET)))
                            (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_SOME_DATA_PO)
                                 (any-to-ram-some-data-first-dt-size))))))

(defconstraint any-to-ram-some-data-mmio-writting (:guard IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA)
  (begin (if-eq NT_FIRST 1
                (begin (will-inc! micro/SLO 1)
                       (vanishes! (next micro/SBO))
                       (if-zero (any-to-ram-some-data-tlo-increment-after-first-dt)
                                (will-remain-constant! micro/TLO)
                                (will-inc! micro/TLO 1))
                       (will-eq! micro/TBO (any-to-ram-some-data-middle-tbo))))
         (if-eq NT_MDDL 1
                (begin (if-eq-else (any-to-ram-some-data-data-src-is-ram) 1
                                   (if-zero (any-to-ram-some-data-aligned)
                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TRANSPLANT))
                                   (if-zero (any-to-ram-some-data-aligned)
                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TRANSPLANT)))
                       (eq! micro/SIZE LLARGE)
                       (will-inc! micro/SLO 1)
                       (vanishes! (next micro/SBO))
                       (will-inc! micro/TLO 1)
                       (will-eq! micro/TBO (any-to-ram-some-data-middle-tbo))))
         (if-eq NT_LAST 1
                (begin (if-eq-else (any-to-ram-some-data-data-src-is-ram) 1
                                   (if-zero (any-to-ram-some-data-last-dt-single-target)
                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_TWO_TARGET)
                                            (eq! micro/INST MMIO_INST_RAM_TO_RAM_PARTIAL))
                                   (if-zero (any-to-ram-some-data-last-dt-single-target)
                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                                            (eq! micro/INST MMIO_INST_LIMB_TO_RAM_ONE_TARGET)))
                       (eq! micro/SIZE (any-to-ram-some-data-last-dt-size))))
         (if-eq (force-bool (+ RZ_FIRST RZ_ONLY)) 1
                (begin (eq! micro/INST MMIO_INST_RAM_EXCISION)
                       (eq! micro/SIZE (any-to-ram-some-data-first-padding-size))
                       (did-inc! micro/TLO (any-to-ram-some-data-tlo-increment-at-transition))
                       (eq! micro/TBO (any-to-ram-some-data-first-pbo))))
         (if-eq (force-bool (+ RZ_FIRST RZ_MDDL)) 1
                (begin (will-inc! micro/TLO 1)
                       (vanishes! (next micro/TBO))))
         (if-eq RZ_MDDL 1 (eq! micro/INST MMIO_INST_RAM_VANISHES))
         (if-eq RZ_LAST 1
                (begin (eq! micro/INST MMIO_INST_RAM_EXCISION)
                       (eq! micro/SIZE (any-to-ram-some-data-last-padding-size))))))

;;
;; MODEXP ZERO
;;
(defconstraint modexp-zero-preprocessing (:guard (* MACRO IS_MODEXP_ZERO))
  (begin (vanishes! TOTLZ)
         (eq! TOTNT NB_MICRO_ROWS_TOT_MODEXP_ZERO)
         (vanishes! TOTRZ)
         (eq! (shift micro/EXO_SUM NB_PP_ROWS_MODEXP_ZERO_PO) EXO_SUM_WEIGHT_BLAKEMODEXP)
         (eq! (shift micro/PHASE NB_PP_ROWS_MODEXP_ZERO_PO) macro/PHASE)
         (eq! (shift micro/EXO_ID NB_PP_ROWS_MODEXP_ZERO_PO) macro/TGT_ID)))

(defconstraint modexp-zero-mmio-instruction-writting (:guard (* MICRO IS_MODEXP_ZERO))
  (begin (stdProgression micro/TLO)
         (eq! micro/INST MMIO_INST_LIMB_VANISHES)))

;;
;; MODEXP DATA
;;
(defun (modexp-initial-tbo)
  [OUT 1])

(defun (modexp-initial-slo)
  [OUT 2])

(defun (modexp-initial-sbo)
  [OUT 3])

(defun (modexp-first-limb-bytesize)
  [OUT 4])

(defun (modexp-last-limb-bytesize)
  [OUT 5])

(defun (modexp-first-limb-single-source)
  [BIN 1])

(defun (modexp-aligned)
  [BIN 2])

(defun (modexp-last-limb-single-source)
  [BIN 3])

(defun (modexp-src-id)
  macro/SRC_ID)

(defun (modexp-tgt-id)
  macro/TGT_ID)

(defun (modexp-src-offset)
  macro/SRC_OFFSET_LO)

(defun (modexp-size)
  macro/SIZE)

(defun (modexp-cdo)
  macro/REF_OFFSET)

(defun (modexp-cds)
  macro/REF_SIZE)

(defun (modexp-exo-sum)
  macro/EXO_SUM)

(defun (modexp-phase)
  macro/PHASE)

(defun (modexp-param-byte-size)
  (modexp-size))

(defun (modexp-param-offset)
  (+ (modexp-cdo) (modexp-src-offset)))

(defun (modexp-leftover-data-size)
  (- (modexp-cds) (modexp-src-offset)))

(defun (modexp-num-left-padding-bytes)
  (- 512 (modexp-param-byte-size)))

(defun (modexp-data-runs-out)
  (shift prprc/WCP_RES 2))

(defun (modexp-num-right-padding-bytes)
  (* (- (modexp-param-byte-size) (modexp-leftover-data-size)) (modexp-data-runs-out)))

(defun (modexp-right-padding-remainder)
  (shift prprc/EUC_REM 2))

(defun (modexp-totnt-is-one)
  (shift prprc/WCP_RES 3))

(defun (modexp-middle-sbo)
  (shift prprc/EUC_REM 6))

(defconstraint modexp-preprocessing (:guard (* MACRO IS_MODEXP_DATA))
  (begin  ;; Setting total number of mmio inst
         (eq! TOT NB_MICRO_ROWS_TOT_MODEXP_DATA)
         ;; preprocessing row n°1
         (callToEuc 1 (modexp-num-left-padding-bytes) LLARGE)
         (eq! (modexp-initial-tbo) (next prprc/EUC_REM))
         (eq! TOTLZ (next prprc/EUC_QUOT))
         ;; preprocessing row n°2
         (callToLt 2 0 (modexp-leftover-data-size) (modexp-param-byte-size))
         (callToEuc 2 (modexp-num-right-padding-bytes) LLARGE)
         (eq! TOTRZ (shift prprc/EUC_QUOT 2))
         (debug (eq! TOTNT
                     (- 32 (+ TOTLZ TOTRZ))))
         ;; preprocessing row n°3
         (callToEq 3 0 TOTNT 1)
         (callToEuc 3 (modexp-param-offset) LLARGE)
         (eq! (modexp-initial-slo) (shift prprc/EUC_QUOT 3))
         (eq! (modexp-initial-sbo) (shift prprc/EUC_REM 3))
         (if-zero (modexp-totnt-is-one)
                  (eq! (modexp-first-limb-bytesize) (- LLARGE (modexp-initial-tbo)))
                  (if-zero (modexp-data-runs-out)
                           (eq! (modexp-first-limb-bytesize) (modexp-param-byte-size))
                           (eq! (modexp-first-limb-bytesize) (modexp-leftover-data-size))))
         (if-zero (modexp-data-runs-out)
                  (eq! (modexp-last-limb-bytesize) LLARGE)
                  (eq! (modexp-last-limb-bytesize) (- LLARGE (modexp-right-padding-remainder))))
         ;; preprocessing row n°4
         (callToLt 4
                   0
                   (+ (modexp-initial-slo) (- (modexp-first-limb-bytesize) 1))
                   LLARGE)
         (eq! (modexp-first-limb-single-source) (shift prprc/WCP_RES 4))
         ;; preprocessing row n°5
         (callToEq 5 0 (modexp-initial-sbo) (modexp-initial-tbo))
         (eq! (modexp-aligned) (shift prprc/WCP_RES 5))
         ;; preprocessing row n°6
         (if-eq-else (modexp-aligned) 1
                     (eq! (modexp-last-limb-single-source) (modexp-aligned))
                     (begin (callToEuc 6 (+ (modexp-initial-sbo) (modexp-first-limb-bytesize)) LLARGE)
                            (callToLt 6
                                      0
                                      (+ (modexp-middle-sbo) (- (modexp-last-limb-bytesize) 1))
                                      LLARGE)
                            (eq! (modexp-last-limb-single-source) (shift prprc/WCP_RES 6))))
         ;; setting mmio constant values
         (eq! (shift micro/CN_S NB_PP_ROWS_MODEXP_DATA_PO) (modexp-src-id))
         (eq! (shift micro/EXO_SUM NB_PP_ROWS_MODEXP_DATA_PO) EXO_SUM_WEIGHT_BLAKEMODEXP)
         (eq! (shift micro/PHASE NB_PP_ROWS_MODEXP_DATA_PO) (modexp-phase))
         (eq! (shift micro/EXO_ID NB_PP_ROWS_MODEXP_DATA_PO) (modexp-tgt-id))))

(defconstraint modexp-mmio-instruction-writting (:guard IS_MODEXP_DATA)
  (begin (if-eq MICRO 1 (stdProgression micro/TLO))
         (if-eq (zero-row) 1 (eq! micro/INST MMIO_INST_LIMB_VANISHES))
         (if-eq (force-bool (+ NT_ONLY NT_FIRST)) 1
                (begin (if-zero (modexp-first-limb-single-source)
                                (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                (eq! micro/INST MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                       (eq! micro/SIZE (modexp-first-limb-bytesize))
                       (eq! micro/SLO (modexp-initial-slo))
                       (eq! micro/SBO (modexp-initial-sbo))
                       (eq! micro/TBO (modexp-initial-tbo))))
         (if-eq NT_FIRST 1
                (begin (if-eq-else (modexp-aligned) 1
                                   (will-inc! micro/SLO 1)
                                   (if-zero (modexp-first-limb-single-source)
                                            (begin (will-inc! micro/SLO 1)
                                                   (will-eq! micro/SBO
                                                             (- (+ micro/SBO micro/SIZE) LLARGE)))
                                            (begin (will-remain-constant! micro/SLO)
                                                   (will-eq! micro/SBO (+ micro/SBO micro/SIZE)))))
                       (vanishes! (next micro/TBO))))
         (if-eq NT_MDDL 1
                (begin (if-zero (modexp-aligned)
                                (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
                       (eq! micro/SIZE LLARGE)
                       (will-inc! micro/SLO 1)
                       (will-remain-constant! micro/SBO)
                       (will-remain-constant! micro/TBO)))
         (if-eq NT_LAST 1
                (begin (if-zero (modexp-last-limb-single-source)
                                (eq! micro/INST MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                (eq! micro/INST MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                       (eq! micro/SIZE (modexp-last-limb-bytesize))))))

;;
;; BLAKE
;;
(defun (blake-cdo)
  macro/SRC_OFFSET_LO)

(defun (blake-success-bit)
  macro/SUCCESS_BIT)

(defun (blake-r-prediction)
  macro/LIMB_1)

(defun (blake-f-prediction)
  macro/LIMB_2)

(defun (blake-slo-r)
  (next prprc/EUC_QUOT))

(defun (blake-sbo-r)
  (next prprc/EUC_REM))

(defun (blake-r-single-source)
  (next prprc/WCP_RES))

(defun (blake-slo-f)
  (shift prprc/EUC_QUOT 2))

(defun (blake-sbo-f)
  (shift prprc/EUC_REM 2))

(defconstraint blake-preprocessing (:guard (* MACRO IS_BLAKE))
  (begin  ;; setiing nb of mmio instruction
         (vanishes! TOTLZ)
         (eq! TOTNT 2)
         (vanishes! TOTRZ)
         ;; preprocessing row n°1
         (callToEuc 1 (blake-cdo) LLARGE)
         (callToLt 1
                   0
                   (+ (blake-sbo-r) (- 4 1))
                   LLARGE)
         ;; preprocessing row n°2
         (callToEuc 2
                    (+ (blake-cdo) (- 213 1))
                    LLARGE)
         ;; mmio constant values
         (eq! (shift micro/CN_S NB_PP_ROWS_BLAKE_PO) macro/SRC_ID)
         (eq! (shift micro/SUCCESS_BIT NB_PP_ROWS_BLAKE_PO) (blake-success-bit))
         (eq! (shift micro/EXO_SUM NB_PP_ROWS_BLAKE_PO)
              (* (blake-success-bit) EXO_SUM_WEIGHT_BLAKEMODEXP))
         (eq! (shift micro/PHASE NB_PP_ROWS_BLAKE_PO) (* (blake-success-bit) PHASE_BLAKE_PARAMS))
         (eq! (shift micro/EXO_ID NB_PP_ROWS_BLAKE_PO) (* (blake-success-bit) macro/TGT_ID))
         ;; first mmio inst
         (if-zero (blake-r-single-source)
                  (eq! (shift micro/INST NB_PP_ROWS_BLAKE_PO) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                  (eq! (shift micro/INST NB_PP_ROWS_BLAKE_PO) MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
         (eq! (shift micro/SIZE NB_PP_ROWS_BLAKE_PO) 4)
         (eq! (shift micro/SLO NB_PP_ROWS_BLAKE_PO) (blake-slo-r))
         (eq! (shift micro/SBO NB_PP_ROWS_BLAKE_PO) (blake-sbo-r))
         (vanishes! (shift micro/TLO NB_PP_ROWS_BLAKE_PO))
         (eq! (shift micro/TBO NB_PP_ROWS_BLAKE_PO) (- LLARGE 4))
         (eq! (shift micro/LIMB NB_PP_ROWS_BLAKE_PO) (blake-r-prediction))
         ;; second mmio inst
         (eq! (shift micro/INST NB_PP_ROWS_BLAKE_PT) MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
         (eq! (shift micro/SIZE NB_PP_ROWS_BLAKE_PT) 1)
         (eq! (shift micro/SLO NB_PP_ROWS_BLAKE_PT) (blake-slo-f))
         (eq! (shift micro/SBO NB_PP_ROWS_BLAKE_PT) (blake-sbo-f))
         (eq! (shift micro/TLO NB_PP_ROWS_BLAKE_PT) 1)
         (eq! (shift micro/TBO NB_PP_ROWS_BLAKE_PT) (- LLARGE 1))
         (eq! (shift micro/LIMB NB_PP_ROWS_BLAKE_PT) (blake-f-prediction))))


