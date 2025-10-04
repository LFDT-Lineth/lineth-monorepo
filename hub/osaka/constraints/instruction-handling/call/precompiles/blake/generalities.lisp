(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.Y.22    BLAKE2fCMUL       ;;
;;    X.Y.22.1  Introduction      ;;
;;    X.Y.22.2  Representation    ;;
;;    X.Y.22.3  Generalities      ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;
;; Shorthands ;;
;;;;;;;;;;;;;;;;


(defun    (precompile-processing---BLAKE2f---standard-precondition)    (*    PEEK_AT_SCENARIO    scenario/PRC_BLAKE2f))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 1 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---BLAKE2f---setting-MISC-flags
                  (:guard    (precompile-processing---BLAKE2f---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   (weighted-MISC-flag-sum-sans-MMU    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction)    MISC_WEIGHT_OOB)
                    (eq!   (shift    misc/MMU_FLAG             precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction)    (precompile-processing---BLAKE2f---OOB-hub-success))
                    ))

(defconstraint    precompile-processing---BLAKE2f---setting-first-OOB-instruction
                  (:guard    (precompile-processing---BLAKE2f---standard-precondition))
                  (set-OOB-instruction---blake-cds     precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction    ;; offset
                                                       (precompile-processing---dup-cds)                                                 ;; call data size
                                                       (precompile-processing---dup-r@c)                                                 ;; return at capacity
                                                       ))


(defconstraint    precompile-processing---BLAKE2f---setting-FAILURE_KNOWN_TO_HUB
                  (:guard    (precompile-processing---BLAKE2f---standard-precondition))
                  (eq!    scenario/PRC_FAILURE_KNOWN_TO_HUB
                          (-    1    (precompile-processing---BLAKE2f---OOB-hub-success))))

(defconstraint    precompile-processing---BLAKE2f---setting-the-MMU-instruction---parameter-instruction
                  (:guard    (precompile-processing---BLAKE2f---standard-precondition))
                  (if-not-zero   (shift    misc/MMU_FLAG         precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction)
                                 (set-MMU-instruction---blake    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction   ;; offset
                                                                 CONTEXT_NUMBER                                                                   ;; source ID
                                                                 (+    1    HUB_STAMP)                                                            ;; target ID
                                                                 ;; aux_id                                                                           ;; auxiliary ID
                                                                 ;; src_offset_hi                                                                    ;; source offset high
                                                                 (precompile-processing---dup-cdo)                                                ;; source offset low
                                                                 ;; tgt_offset_lo                                                                    ;; target offset low
                                                                 ;; size                                                                             ;; size
                                                                 ;; ref_offset                                                                       ;; reference offset
                                                                 ;; ref_size                                                                         ;; reference size
                                                                 ;; (scenario-shorthand---PRC---success)                                             ;; success bit
                                                                 ;; limb_1                                                                           ;; limb 1
                                                                 ;; limb_2                                                                           ;; limb 2
                                                                 ;; exo_sum                                                                          ;; weighted exogenous module flag sum
                                                                 ;; phase                                                                            ;; phase
                                                                 )))

(defconstraint    precompile-processing---BLAKE2f---constraining-the-call-success
                  (:guard         (precompile-processing---BLAKE2f---standard-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG                    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction)
                                  (eq!     (shift    misc/MMU_SUCCESS_BIT    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction)
                                           (scenario-shorthand---PRC---success))))


;;;;;;;;;;;;;;;;;;;;;
;; More shorthands ;;
;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---BLAKE2f---r-parameter)        (shift     misc/MMU_LIMB_1       precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction))
(defun    (precompile-processing---BLAKE2f---f-parameter)        (shift     misc/MMU_LIMB_2       precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction))
(defun    (precompile-processing---BLAKE2f---OOB-hub-success)    (shift    [misc/OOB_DATA   4]    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction))
(defun    (precompile-processing---BLAKE2f---OOB-r@c-nonzero)    (shift    [misc/OOB_DATA   8]    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-parameter-extraction))
