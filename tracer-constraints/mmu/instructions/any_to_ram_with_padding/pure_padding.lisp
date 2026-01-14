(module mmu)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;  MMU Instructions  ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;   ANY_TO_RAM_WITH_PADDING   ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;       pure padding case     ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (any-to-ram-with-padding---pure-padding---last-padding-is-full)      [BIN 1])
(defun    (any-to-ram-with-padding---pure-padding---last-padding-size)         [OUT 1]) ;; ""
(defun    (any-to-ram-with-padding---pure-padding---totrz-is-one)              (shift prprc/WCP_RES 3))
(defun    (any-to-ram-with-padding---pure-padding---first-padding-is-full)     (shift prprc/WCP_RES 4))
(defun    (any-to-ram-with-padding---pure-padding---only-padding-is-full)      (* (any-to-ram-with-padding---pure-padding---first-padding-is-full) (any-to-ram-with-padding---pure-padding---last-padding-is-full)))
(defun    (any-to-ram-with-padding---pure-padding---first-padding-size)        (- LLARGE (any-to-ram-with-padding---min-tbo)))
(defun    (any-to-ram-with-padding---pure-padding---only-padding-size)         (any-to-ram-with-padding---padd-size))

(defconstraint    any-to-ram-with-padding---pure-padding---setting-the-TOTs (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
                  (begin
                    ;; setting number of rows
                    (vanishes! TOTLZ)
                    (vanishes! TOTNT)
                    (eq!       TOTRZ (+ (- (any-to-ram-with-padding---max-tlo) (any-to-ram-with-padding---min-tlo)) 1))))

(defconstraint    any-to-ram-with-padding---pure-padding---3rd-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
                  ;; preprocessing row n°3
                  (callToEq     3
                                0
                                TOTRZ
                                1))

(defconstraint    any-to-ram-with-padding---pure-padding---4th-preprocessing-row (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
                  (begin
                    ;; preprocessing row n°4
                    (callToIszero 4
                                  0
                                  (any-to-ram-with-padding---min-tbo))
                    (callToEuc    4
                                  (+ 1 (any-to-ram-with-padding---max-tbo))
                                  LLARGE)
                    (eq! (any-to-ram-with-padding---pure-padding---last-padding-is-full) (* (- 1 (any-to-ram-with-padding---pure-padding---totrz-is-one)) (shift prprc/EUC_QUOT 4)))
                    (eq! (any-to-ram-with-padding---pure-padding---last-padding-size)    (* (- 1 (any-to-ram-with-padding---pure-padding---totrz-is-one)) (+ 1 (any-to-ram-with-padding---max-tbo))))))

(defconstraint    any-to-ram-with-padding---pure-padding---setting-micro-instruction-constant-values (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
                  ;; mmio constant values
                  (eq! (shift micro/CN_T NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO) macro/TGT_ID))

(defconstraint    any-to-ram-with-padding---pure-padding---initializing-tlo-tbo (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
                  (begin
                    ;; first and only common mmio
                    (eq! (shift micro/TLO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO) (any-to-ram-with-padding---min-tlo))
                    (eq! (shift micro/TBO NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO) (any-to-ram-with-padding---min-tbo))))

(defconstraint    any-to-ram-with-padding---pure-padding---FIRST-and-ONLY-micro-instruction-writing (:guard (* MACRO IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING))
                  (if-zero (any-to-ram-with-padding---pure-padding---totrz-is-one)
                           ;; first mmio instruction
                           (begin (if-zero (any-to-ram-with-padding---pure-padding---first-padding-is-full)
                                           (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                                MMIO_INST_RAM_EXCISION)
                                           (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                                MMIO_INST_RAM_VANISHES))
                                  (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                       (any-to-ram-with-padding---pure-padding---first-padding-size)))
                           ;; only mmio instruction
                           (begin (if-zero (any-to-ram-with-padding---pure-padding---only-padding-is-full)
                                           (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                                MMIO_INST_RAM_EXCISION)
                                           (eq! (shift micro/INST NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                                MMIO_INST_RAM_VANISHES))
                                  (eq! (shift micro/SIZE NB_PP_ROWS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING_PO)
                                       (any-to-ram-with-padding---pure-padding---only-padding-size)))))

(defconstraint    any-to-ram-with-padding---pure-padding---paying-forward (:guard IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)
                  (if-eq (force-bin (+ RZ_MDDL RZ_LAST)) 1
                         (begin (did-inc!  micro/TLO 1)
                                (vanishes! micro/TBO))))

(defconstraint    any-to-ram-with-padding---pure-padding---MDDL-and-LAST-micro-instruction-writing (:guard IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING)
                  (begin
                    ;; middle mmio instructions
                    (if-eq RZ_MDDL 1 (eq! micro/INST MMIO_INST_RAM_VANISHES))
                    ;; last mmio instruction
                    (if-eq RZ_LAST 1
                           (begin (if-zero (any-to-ram-with-padding---pure-padding---last-padding-is-full)
                                           (eq! micro/INST MMIO_INST_RAM_EXCISION)
                                           (eq! micro/INST MMIO_INST_RAM_VANISHES))
                                  (eq! micro/SIZE (any-to-ram-with-padding---pure-padding---last-padding-size))))))
