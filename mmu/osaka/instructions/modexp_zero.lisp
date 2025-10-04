(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;   MODEXP_ZERO   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    modexp-zero---setting-the-TOTs (:guard (* MACRO IS_MODEXP_ZERO))
                  (begin
                    (vanishes! TOTLZ)
                    (eq!       TOTNT NB_MICRO_ROWS_TOT_MODEXP_ZERO)
                    (vanishes! TOTRZ)))

(defconstraint    modexp-zero---setting-micro-instruction-constant-values (:guard (* MACRO IS_MODEXP_ZERO))
                  (begin
                    (eq!    (shift    micro/EXO_SUM    NB_PP_ROWS_MODEXP_ZERO_PO)    EXO_SUM_WEIGHT_BLAKEMODEXP)
                    (eq!    (shift    micro/PHASE      NB_PP_ROWS_MODEXP_ZERO_PO)    macro/PHASE)
                    (eq!    (shift    micro/EXO_ID     NB_PP_ROWS_MODEXP_ZERO_PO)    macro/TGT_ID)))

(defconstraint    modexp-zero---mmio-instruction-writting (:guard (* MICRO IS_MODEXP_ZERO))
                  (begin
                    (standard-progression    micro/TLO)
                    (eq!    micro/INST    MMIO_INST_LIMB_VANISHES)))

