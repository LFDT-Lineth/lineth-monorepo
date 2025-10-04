(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;            ;;
;;;;;;;;;;;;;;;;;;;;;;;;   MSTORE   ;;
;;;;;;;;;;;;;;;;;;;;;;;;            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (mstore---aligned)    (next prprc/WCP_RES))
(defun    (mstore---tlo)        (next prprc/EUC_QUOT))
(defun    (mstore---tbo)        (next prprc/EUC_REM))

(defconstraint    mstore---setting-the-TOTs (:guard (* MACRO IS_MSTORE))
                  (begin
                    (vanishes! TOTLZ)
                    (eq!       TOTNT NB_MICRO_ROWS_TOT_MSTORE)
                    (vanishes! TOTRZ)))

(defconstraint    mstore---pre-processing (:guard (* MACRO IS_MSTORE))
                  (begin
                    (callToEuc    1
                                  macro/TGT_OFFSET_LO
                                  LLARGE)
                    (callToIszero 1
                                  0
                                  (mstore---tbo))))

(defconstraint    mstore---setting-micro-instruction-constant-values (:guard (* MACRO IS_MSTORE))
                  (begin
                    (eq!       (shift micro/CN_T NB_PP_ROWS_MSTORE_PO) macro/TGT_ID)
                    (vanishes! (shift micro/EXO_SUM NB_PP_ROWS_MSTORE_PO))))

(defconstraint    mstore---1st-micro-instruction-writing (:guard (* MACRO IS_MSTORE))
                  (begin
                    (if-zero (mstore---aligned)
                             (eq!    (shift    micro/INST    NB_PP_ROWS_MSTORE_PO)    MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                             (eq!    (shift    micro/INST    NB_PP_ROWS_MSTORE_PO)    MMIO_INST_LIMB_TO_RAM_TRANSPLANT))
                    (eq!             (shift    micro/SIZE    NB_PP_ROWS_MSTORE_PO)    LLARGE)
                    (eq!             (shift    micro/TLO     NB_PP_ROWS_MSTORE_PO)    (mstore---tlo))
                    (eq!             (shift    micro/TBO     NB_PP_ROWS_MSTORE_PO)    (mstore---tbo))
                    (eq!             (shift    micro/LIMB    NB_PP_ROWS_MSTORE_PO)    macro/LIMB_1)))

(defconstraint    mstore---2nd-micro-instruction-writing (:guard (* MACRO IS_MSTORE))
                  (begin
                    (if-zero (mstore---aligned)
                             (eq!    (shift    micro/INST    NB_PP_ROWS_MSTORE_PT)    MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
                             (eq!    (shift    micro/INST    NB_PP_ROWS_MSTORE_PT)    MMIO_INST_LIMB_TO_RAM_TRANSPLANT))
                    (eq!             (shift    micro/SIZE    NB_PP_ROWS_MSTORE_PT)    LLARGE)
                    (eq!             (shift    micro/TLO     NB_PP_ROWS_MSTORE_PT)    (+ (mstore---tlo) 1))
                    (eq!             (shift    micro/TBO     NB_PP_ROWS_MSTORE_PT)    (mstore---tbo))
                    (eq!             (shift    micro/LIMB    NB_PP_ROWS_MSTORE_PT)    macro/LIMB_2)))
