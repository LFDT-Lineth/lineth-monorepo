(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;             ;;
;;;;;;;;;;;;;;;;;;;;;;;;   MSTORE8   ;;
;;;;;;;;;;;;;;;;;;;;;;;;             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (mstore8---tlo)    (next prprc/EUC_QUOT))
(defun    (mstore8---tbo)    (next prprc/EUC_REM))

(defconstraint    mstore8---setting-the-TOTs (:guard (* MACRO IS_MSTORE8))
                  (begin
                    (vanishes! TOTLZ)
                    (eq!       TOTNT NB_MICRO_ROWS_TOT_MSTORE_EIGHT)
                    (vanishes! TOTRZ)))

(defconstraint    mstore8---pre-processing (:guard (* MACRO IS_MSTORE8))
                  (begin
                    (callToEuc 1 macro/TGT_OFFSET_LO LLARGE)))

(defconstraint    mstore8---micro-instruction-writing (:guard (* MACRO IS_MSTORE8))
                  (begin
                    (eq!         (shift    micro/INST       NB_PP_ROWS_MSTORE8_PO) MMIO_INST_LIMB_TO_RAM_ONE_TARGET)
                    (eq!         (shift    micro/SIZE       NB_PP_ROWS_MSTORE8_PO) 1)
                    (eq!         (shift    micro/SBO        NB_PP_ROWS_MSTORE8_PO) LLARGEMO)
                    (eq!         (shift    micro/TLO        NB_PP_ROWS_MSTORE8_PO) (mstore8---tlo))
                    (eq!         (shift    micro/TBO        NB_PP_ROWS_MSTORE8_PO) (mstore8---tbo))
                    (eq!         (shift    micro/LIMB       NB_PP_ROWS_MSTORE8_PO) macro/LIMB_2)
                    (eq!         (shift    micro/CN_T       NB_PP_ROWS_MSTORE8_PO) macro/TGT_ID)
                    (vanishes!   (shift    micro/EXO_SUM    NB_PP_ROWS_MSTORE8_PO))))
