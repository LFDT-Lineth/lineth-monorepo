(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;   EXO_TO_RAM_TRANSPLANTS   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    exo-to-ram-transplant---preprocessing (:guard (* IS_EXO_TO_RAM_TRANSPLANTS MACRO))
                  ;; setting prprc row n°1
                  (callToEuc    1
                                macro/SIZE
                                LLARGE))

(defconstraint    exo-to-ram-transplant---setting-the-TOTs (:guard (* IS_EXO_TO_RAM_TRANSPLANTS MACRO))
                  (begin
                    ;; setting nb of rows
                    (vanishes! TOTLZ)
                    (eq!       TOTNT (next prprc/EUC_CEIL))
                    (vanishes! TOTRZ)))

(defconstraint    exo-to-ram-transplant---setting-micro-instruction-constant-values (:guard (* IS_EXO_TO_RAM_TRANSPLANTS MACRO))
                  (begin
                    ;; setting mmio constant values
                    (eq!    (shift    micro/CN_T        NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO)    macro/TGT_ID)
                    (eq!    (shift    micro/EXO_SUM     NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO)    macro/EXO_SUM)
                    (eq!    (shift    micro/PHASE       NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO)    macro/PHASE)
                    (eq!    (shift    micro/EXO_ID      NB_PP_ROWS_EXO_TO_RAM_TRANSPLANTS_PO)    macro/SRC_ID)))

(defconstraint    exo-to-ram-transplant---micro-instruction-writing (:guard (* IS_EXO_TO_RAM_TRANSPLANTS MICRO))
                  (begin (standard-progression    micro/SLO)
                         (standard-progression    micro/TLO)
                         (eq!    micro/INST    MMIO_INST_LIMB_TO_RAM_TRANSPLANT)))
