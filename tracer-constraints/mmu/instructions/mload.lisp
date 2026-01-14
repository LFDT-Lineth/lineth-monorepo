(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;           ;;
;;;;;;;;;;;;;;;;;;;;;;;;   MLOAD   ;;
;;;;;;;;;;;;;;;;;;;;;;;;           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (mload---aligned)    (next prprc/WCP_RES))
(defun    (mload---slo)        (next prprc/EUC_QUOT))
(defun    (mload---sbo)        (next prprc/EUC_REM))

(defconstraint    mload---setting-the-TOTs    (:guard (* MACRO IS_MLOAD))
                  (begin
                    (vanishes! TOTLZ)
                    (eq!       TOTNT    NB_MICRO_ROWS_TOT_MLOAD)
                    (vanishes! TOTRZ)))

(defconstraint    mload---pre-processing (:guard (* MACRO IS_MLOAD))
                  (begin
                    (callToEuc    1
                                  macro/SRC_OFFSET_LO
                                  LLARGE)
                    (callToIszero 1
                                  0
                                  (mload---sbo))))

(defconstraint    mload---setting-micro-instruction-constant-values    (:guard (* MACRO IS_MLOAD))
                  (begin
                    (eq!       (shift micro/CN_S       NB_PP_ROWS_MLOAD_PO) macro/SRC_ID)
                    (vanishes! (shift micro/EXO_SUM    NB_PP_ROWS_MLOAD_PO))))

(defconstraint    mload---1st-micro-instruction-writing (:guard (* MACRO IS_MLOAD))
                  (begin
                    (if-zero     (mload---aligned)
                                 (eq!    (shift    micro/INST    NB_PP_ROWS_MLOAD_PO) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                 (eq!    (shift    micro/INST    NB_PP_ROWS_MLOAD_PO) MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
                    (eq!                 (shift    micro/SIZE    NB_PP_ROWS_MLOAD_PO) LLARGE)
                    (eq!                 (shift    micro/SLO     NB_PP_ROWS_MLOAD_PO) (mload---slo))
                    (eq!                 (shift    micro/SBO     NB_PP_ROWS_MLOAD_PO) (mload---sbo))
                    (vanishes!           (shift    micro/TBO     NB_PP_ROWS_MLOAD_PO))
                    (eq!                 (shift    micro/LIMB    NB_PP_ROWS_MLOAD_PO) macro/LIMB_1)))

(defconstraint    mload---2nd-micro-instruction-writing (:guard (* MACRO IS_MLOAD))
                  (begin
                    (if-zero     (mload---aligned)
                                 (eq!    (shift    micro/INST    NB_PP_ROWS_MLOAD_PT) MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
                                 (eq!    (shift    micro/INST    NB_PP_ROWS_MLOAD_PT) MMIO_INST_RAM_TO_LIMB_TRANSPLANT))
                    (eq!                 (shift    micro/SIZE    NB_PP_ROWS_MLOAD_PT) LLARGE)
                    (eq!                 (shift    micro/SLO     NB_PP_ROWS_MLOAD_PT) (+ (mload---slo) 1))
                    (eq!                 (shift    micro/SBO     NB_PP_ROWS_MLOAD_PT) (mload---sbo))
                    (vanishes!           (shift    micro/TBO     NB_PP_ROWS_MLOAD_PT))
                    (eq!                 (shift    micro/LIMB    NB_PP_ROWS_MLOAD_PT) macro/LIMB_2)))
