(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;           ;;
;;;;;;;;;;;;;;;;;;;;;;;;   BLAKE   ;;
;;;;;;;;;;;;;;;;;;;;;;;;           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (blake---cdo)                 macro/SRC_OFFSET_LO)
(defun    (blake---success-bit)         macro/SUCCESS_BIT)
(defun    (blake---r-prediction)        macro/LIMB_1)
(defun    (blake---f-prediction)        macro/LIMB_2)
(defun    (blake---slo-r)              (next prprc/EUC_QUOT))
(defun    (blake---sbo-r)              (next prprc/EUC_REM))
(defun    (blake---r-single-source)    (next prprc/WCP_RES))
(defun    (blake---slo-f)              (shift prprc/EUC_QUOT 2))
(defun    (blake---sbo-f)              (shift prprc/EUC_REM 2)) ;; ""

(defconstraint    blake---setting-the-TOTs (:guard (* MACRO IS_BLAKE))
                  (begin
                    ;; setting nb of mmio instruction
                    (vanishes! TOTLZ)
                    (eq!       TOTNT 2)
                    (vanishes! TOTRZ)))

(defconstraint    blake---1st-processing-row (:guard (* MACRO IS_BLAKE))
                  (begin
                    ;; preprocessing row n°1
                    (callToEuc    1
                                  (blake---cdo)
                                  LLARGE)
                    (callToLt     1
                                  0
                                  (+ (blake---sbo-r) (- 4 1))
                                  LLARGE)))

(defconstraint    blake---2nd-processing-row (:guard (* MACRO IS_BLAKE))
                  ;; preprocessing row n°2
                  (callToEuc    2
                                (+ (blake---cdo) (- PRECOMPILE_CALL_DATA_SIZE___BLAKE2F 1))
                                LLARGE))

(defconstraint    blake---setting-micro-instruction-constant-values (:guard (* MACRO IS_BLAKE))
                  (begin
                    ;; mmio constant values
                    (eq!    (shift micro/CN_S           NB_PP_ROWS_BLAKE_PO)    macro/SRC_ID)
                    (eq!    (shift micro/SUCCESS_BIT    NB_PP_ROWS_BLAKE_PO)    (blake---success-bit))
                    (eq!    (shift micro/EXO_SUM        NB_PP_ROWS_BLAKE_PO)    (* (blake---success-bit) EXO_SUM_WEIGHT_BLAKEMODEXP))
                    (eq!    (shift micro/PHASE          NB_PP_ROWS_BLAKE_PO)    (* (blake---success-bit) PHASE_BLAKE_PARAMS))
                    (eq!    (shift micro/EXO_ID         NB_PP_ROWS_BLAKE_PO)    (* (blake---success-bit) macro/TGT_ID))))

(defconstraint    blake---1st-micro-instruction-writing (:guard (* MACRO IS_BLAKE))
                  (begin
                    ;; first mmio inst
                    (if-zero    (blake---r-single-source)
                                (eq!    (shift    micro/INST    NB_PP_ROWS_BLAKE_PO)    MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                (eq!    (shift    micro/INST    NB_PP_ROWS_BLAKE_PO)    MMIO_INST_RAM_TO_LIMB_ONE_SOURCE))
                    (eq!        (shift    micro/SIZE    NB_PP_ROWS_BLAKE_PO) 4)
                    (eq!        (shift    micro/SLO     NB_PP_ROWS_BLAKE_PO) (blake---slo-r))
                    (eq!        (shift    micro/SBO     NB_PP_ROWS_BLAKE_PO) (blake---sbo-r))
                    (vanishes!  (shift    micro/TLO     NB_PP_ROWS_BLAKE_PO))
                    (eq!        (shift    micro/TBO     NB_PP_ROWS_BLAKE_PO) (- LLARGE 4))
                    (eq!        (shift    micro/LIMB    NB_PP_ROWS_BLAKE_PO) (blake---r-prediction))))

(defconstraint    blake--2nd-micro-instruction-writing (:guard (* MACRO IS_BLAKE))
                  (begin
                    ;; second mmio inst
                    (eq!        (shift    micro/INST    NB_PP_ROWS_BLAKE_PT) MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
                    (eq!        (shift    micro/SIZE    NB_PP_ROWS_BLAKE_PT) 1)
                    (eq!        (shift    micro/SLO     NB_PP_ROWS_BLAKE_PT) (blake---slo-f))
                    (eq!        (shift    micro/SBO     NB_PP_ROWS_BLAKE_PT) (blake---sbo-f))
                    (eq!        (shift    micro/TLO     NB_PP_ROWS_BLAKE_PT) 1)
                    (eq!        (shift    micro/TBO     NB_PP_ROWS_BLAKE_PT) (- LLARGE 1))
                    (eq!        (shift    micro/LIMB    NB_PP_ROWS_BLAKE_PT) (blake---f-prediction))))
