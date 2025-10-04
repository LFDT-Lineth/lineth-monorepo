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
;;;;;;;;;;;;;;;;;;;;;;;;       common constraints    ;;
;;;;;;;;;;;;;;;;;;;;;;;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (any-to-ram-with-padding---min-tgt-offset)             macro/TGT_OFFSET_LO)
(defun    (any-to-ram-with-padding---max-tgt-offset)             (+ macro/TGT_OFFSET_LO (- macro/SIZE 1)))
(defun    (any-to-ram-with-padding---pure-padd)                  (force-bin (- 1 (next prprc/WCP_RES))))
(defun    (any-to-ram-with-padding---min-tlo)                    (next prprc/EUC_QUOT))
(defun    (any-to-ram-with-padding---min-tbo)                    (next prprc/EUC_REM))
(defun    (any-to-ram-with-padding---max-src-offset-or-zero)     (* (- 1 (any-to-ram-with-padding---pure-padd))
                                                                    (+ macro/SRC_OFFSET_LO (- macro/SIZE 1))))
(defun    (any-to-ram-with-padding---mixed)                      (force-bin (* (- 1 (any-to-ram-with-padding---pure-padd))
                                                                               (- 1 (shift prprc/WCP_RES 2)))))
(defun    (any-to-ram-with-padding---pure-data)                  (force-bin (* (- 1 (any-to-ram-with-padding---pure-padd)) (shift prprc/WCP_RES 2))))
(defun    (any-to-ram-with-padding---max-tlo)                    (shift prprc/EUC_QUOT 2))
(defun    (any-to-ram-with-padding---max-tbo)                    (shift prprc/EUC_REM 2))
(defun    (any-to-ram-with-padding---trsf-size)                  (+ (* (any-to-ram-with-padding---mixed) (- macro/REF_SIZE macro/SRC_OFFSET_LO))
                                                                    (* (any-to-ram-with-padding---pure-data) macro/SIZE)))
(defun    (any-to-ram-with-padding---padd-size)                  (+ (* (any-to-ram-with-padding---pure-padd) macro/SIZE)
                                                                    (* (any-to-ram-with-padding---mixed)
                                                                       (- macro/SIZE (- macro/REF_SIZE macro/SRC_OFFSET_LO)))))  ;; ""

(defconstraint    any-to-ram-with-padding---common---1st-preprocessing-row (:guard (* MACRO (is-any-to-ram-with-padding)))
                  (begin
                    ;; preprocessing row n°1
                    (callToLt  1
                               macro/SRC_OFFSET_HI
                               macro/SRC_OFFSET_LO
                               macro/REF_SIZE)
                    (callToEuc 1
                               (any-to-ram-with-padding---min-tgt-offset)
                               LLARGE)))

(defconstraint    any-to-ram-with-padding---common---2nd-preprocessing-row (:guard (* MACRO (is-any-to-ram-with-padding)))
                  (begin
                    ;; preprocessing row n°2
                    (callToLt  2
                               0
                               (any-to-ram-with-padding---max-src-offset-or-zero)
                               macro/REF_SIZE)
                    (callToEuc 2
                               (any-to-ram-with-padding---max-tgt-offset)
                               LLARGE)))

(defconstraint    any-to-ram-with-padding---common---justifing-the-pure-padding-vs-some-data-flag (:guard (* MACRO (is-any-to-ram-with-padding)))
                  (begin
                    ;; justifyng the flag
                    (eq! IS_ANY_TO_RAM_WITH_PADDING_PURE_PADDING (any-to-ram-with-padding---pure-padd))
                    (eq! IS_ANY_TO_RAM_WITH_PADDING_SOME_DATA (+ (any-to-ram-with-padding---mixed) (any-to-ram-with-padding---pure-data)))))
