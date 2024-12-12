(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;   RIGHT_PADDED_WORD_EXTRACTION   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (right-padded-word-extraction---second-limb-padded)           (force-bool (- 1 (next prprc/WCP_RES))))
(defun    (right-padded-word-extraction---extract-size)                 (+    (* (right-padded-word-extraction---second-limb-padded) (- macro/REF_SIZE macro/SRC_OFFSET_LO))
                                                                              (* (- 1 (right-padded-word-extraction---second-limb-padded)) WORD_SIZE)))
(defun    (right-padded-word-extraction---first-limb-padded)            (shift prprc/WCP_RES 2))
(defun    (right-padded-word-extraction---second-limb-byte-size)        (+    (* (- 1 (right-padded-word-extraction---second-limb-padded)) LLARGE)
                                                                              (* (right-padded-word-extraction---second-limb-padded)
                                                                                 (+ (* (- 1 (right-padded-word-extraction---first-limb-padded))
                                                                                       (- (right-padded-word-extraction---extract-size) LLARGE))
                                                                                    (* (right-padded-word-extraction---first-limb-padded) 0)))))
(defun    (right-padded-word-extraction---first-limb-byte-size)         (+    (* (- 1 (right-padded-word-extraction---first-limb-padded)) LLARGE)
                                                                              (* (right-padded-word-extraction---first-limb-padded) (right-padded-word-extraction---extract-size))))
(defun    (right-padded-word-extraction---first-limb-is-full)           (force-bool (shift prprc/EUC_QUOT 2)))
(defun    (right-padded-word-extraction---aligned)                      (next    prprc/WCP_RES))
(defun    (right-padded-word-extraction---slo)                          (shift   prprc/EUC_QUOT   3))
(defun    (right-padded-word-extraction---sbo)                          (shift   prprc/EUC_REM    3))
(defun    (right-padded-word-extraction---first-limb-single-source)     (shift   prprc/WCP_RES    3))
(defun    (right-padded-word-extraction---second-limb-single-source)    (shift   prprc/WCP_RES    4))
(defun    (right-padded-word-extraction---second-limb-void)             (shift   prprc/WCP_RES    5))

(defconstraint right-padded-word-extraction---setting-the-TOTs (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 (vanishes! TOTLZ)
                 (eq!       TOTNT NB_MICRO_ROWS_TOT_RIGHT_PADDED_WORD_EXTRACTION)
                 (vanishes! TOTRZ)))

(defconstraint right-padded-word-extraction---1st-pre-processing-row (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting prprc row n°1
                 (callToLt    1
                              0
                              (+ macro/SRC_OFFSET_LO WORD_SIZE)
                              macro/REF_SIZE)))

(defconstraint right-padded-word-extraction---2nd-pre-processing-row (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting prprc row n°2
                 (callToLt    2
                              0
                              (right-padded-word-extraction---extract-size)
                              LLARGE)
                 (callToEuc   2
                              (right-padded-word-extraction---first-limb-byte-size)
                              LLARGE)))

(defconstraint right-padded-word-extraction---3rd-pre-processing-row (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting prprc row n°3
                 (callToEuc   3
                              (+ macro/SRC_OFFSET_LO macro/REF_OFFSET)
                              LLARGE)
                 (callToLt    3
                              0
                              (+ (right-padded-word-extraction---sbo) (right-padded-word-extraction---first-limb-byte-size))
                              LLARGEPO)))

(defconstraint right-padded-word-extraction---4th-pre-processing-row (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting prprc row n°4
                 (callToLt    4
                              0
                              (+ (right-padded-word-extraction---sbo) (right-padded-word-extraction---second-limb-byte-size))
                              LLARGEPO)))

(defconstraint right-padded-word-extraction---5th-pre-processing-row (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting prprc row n°5
                 (callToIszero    5
                                  0
                                  (right-padded-word-extraction---second-limb-byte-size))))

(defconstraint right-padded-word-extraction---setting-micro-instruction-constant-values (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 (eq!       (shift micro/CN_S       NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) macro/SRC_ID)
                 (vanishes! (shift micro/EXO_SUM    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO))))

(defconstraint right-padded-word-extraction---1st-micro-instruction-writing (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting first mmio inst
                 (if-zero   (right-padded-word-extraction---first-limb-single-source)
                            (eq!     (shift    micro/INST    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                            (if-zero (right-padded-word-extraction---first-limb-is-full)
                                     (eq!   (shift   micro/INST   NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO)   MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
                                     (eq!   (shift   micro/INST   NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO)   MMIO_INST_RAM_TO_LIMB_TRANSPLANT)))
                 (eq!       (shift    micro/SIZE    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) (right-padded-word-extraction---first-limb-byte-size))
                 (eq!       (shift    micro/SLO     NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) (right-padded-word-extraction---slo))
                 (eq!       (shift    micro/SBO     NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) (right-padded-word-extraction---sbo))
                 (vanishes! (shift    micro/TBO     NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO))
                 (eq!       (shift    micro/LIMB    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PO) macro/LIMB_1)))

(defconstraint right-padded-word-extraction---2nd-micro-instruction-writing (:guard (* MACRO IS_RIGHT_PADDED_WORD_EXTRACTION))
               (begin
                 ;; setting second mmio inst
                 (if-eq-else    (right-padded-word-extraction---second-limb-void) 1
                                (begin (eq!       (shift micro/INST NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) MMIO_INST_LIMB_VANISHES)
                                       (vanishes! macro/LIMB_2))
                                (if-eq-else (right-padded-word-extraction---second-limb-single-source) 1
                                            (if-zero (right-padded-word-extraction---second-limb-padded)
                                                     (eq!    (shift    micro/INST    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)    MMIO_INST_RAM_TO_LIMB_TRANSPLANT)
                                                     (eq!    (shift    micro/INST    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)    MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                                            (eq!     (shift            micro/INST    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT)    MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)))
                 (eq!           (shift    micro/SIZE    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) (right-padded-word-extraction---second-limb-byte-size))
                 (eq!           (shift    micro/SLO     NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) (+ (right-padded-word-extraction---slo) 1))
                 (eq!           (shift    micro/SBO     NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) (right-padded-word-extraction---sbo))
                 (vanishes!     (shift    micro/TBO     NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT))
                 (eq!           (shift    micro/LIMB    NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION_PT) macro/LIMB_2)))
