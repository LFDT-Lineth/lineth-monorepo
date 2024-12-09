(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.Y.Z.4 MODEXP common processing    ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---MODEXP---standard-precondition)    (*    PEEK_AT_SCENARIO    scenario/PRC_MODEXP))

(defconstraint    precompile-processing---MODEXP---excluding-execution-scenarios    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (vanishes!    scenario/PRC_FAILURE_KNOWN_TO_HUB))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    CALL_DATA_SIZE analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    precompile-processing---MODEXP---cds-misc-row---setting-module-flags       (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---cds-analysis)
                          MISC_WEIGHT_OOB))

(defconstraint    precompile-processing---MODEXP---cds-misc-row---setting-OOB-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (set-OOB-instruction---modexp-cds    precompile-processing---MODEXP---misc-row-offset---cds-analysis   ;; row offset
                                                       (precompile-processing---dup-cds)))                               ;; call data size

(defun    (precompile-processing---MODEXP---extract-bbs)    (shift    [misc/OOB_DATA  3]    precompile-processing---MODEXP---misc-row-offset---cds-analysis))
(defun    (precompile-processing---MODEXP---extract-ebs)    (shift    [misc/OOB_DATA  4]    precompile-processing---MODEXP---misc-row-offset---cds-analysis))
(defun    (precompile-processing---MODEXP---extract-mbs)    (shift    [misc/OOB_DATA  5]    precompile-processing---MODEXP---misc-row-offset---cds-analysis)) ;; ""


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    bbs extraction and analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---bbs-analysis---setting-misc-module-flags    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---bbs-analysis)
                          (+    (*    MISC_WEIGHT_MMU    (precompile-processing---MODEXP---extract-bbs))
                                MISC_WEIGHT_OOB)
                          ))

(defconstraint    precompile-processing---MODEXP---bbs-analysis---setting-MMU-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---bbs-analysis)
                                  (set-MMU-instruction---right-padded-word-extraction    precompile-processing---MODEXP---misc-row-offset---bbs-analysis                                          ;; offset
                                                                                         CONTEXT_NUMBER                                                                                           ;; source ID
                                                                                         ;; tgt_id                                                                                                   ;; target ID
                                                                                         ;; aux_id                                                                                                   ;; auxiliary ID
                                                                                         ;; src_offset_hi                                                                                            ;; source offset high
                                                                                         0                                                                                                        ;; source offset low
                                                                                         ;; tgt_offset_lo                                                                                            ;; target offset low
                                                                                         ;; size                                                                                                     ;; size
                                                                                         (precompile-processing---dup-cdo)                                                                        ;; reference offset
                                                                                         (precompile-processing---dup-cds)                                                                        ;; reference size
                                                                                         ;; success_bit                                                                                              ;; success bit
                                                                                         (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---bbs-analysis)            ;; limb 1          ;; TODO: remove SELF REFERENCE
                                                                                         (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---bbs-analysis)            ;; limb 2          ;; TODO: remove SELF REFERENCE
                                                                                         ;; exo_sum                                                                                                  ;; weighted exogenous module flag sum
                                                                                         ;; phase                                                                                                    ;; phase
                                                                                         )))

(defun    (precompile-processing---MODEXP---bbs-hi)    (*    (precompile-processing---MODEXP---extract-bbs)    (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---bbs-analysis)))
(defun    (precompile-processing---MODEXP---bbs-lo)    (*    (precompile-processing---MODEXP---extract-bbs)    (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---bbs-analysis)))

(defconstraint    precompile-processing---MODEXP---bbs-analysis---setting-OOB-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (set-OOB-instruction---modexp-xbs    precompile-processing---MODEXP---misc-row-offset---bbs-analysis         ;; offset
                                                       (precompile-processing---MODEXP---bbs-hi)                               ;; high part of some {b,e,m}bs
                                                       (precompile-processing---MODEXP---bbs-lo)                               ;; low  part of some {b,e,m}bs
                                                       0                                                                       ;; low  part of some {b,e,m}bs
                                                       0                                                                       ;; bit indicating whether to compute max(xbs, ybs) or not
                                                       ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    ebs extraction and analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---ebs-analysis---setting-misc-module-flags    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---ebs-analysis)
                          (+    (*    MISC_WEIGHT_MMU    (precompile-processing---MODEXP---extract-ebs))
                                MISC_WEIGHT_OOB)
                          ))

(defconstraint    precompile-processing---MODEXP---ebs-analysis---setting-MMU-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---ebs-analysis)
                                  (set-MMU-instruction---right-padded-word-extraction    precompile-processing---MODEXP---misc-row-offset---ebs-analysis                                          ;; offset
                                                                                         CONTEXT_NUMBER                                                                                           ;; source ID
                                                                                         ;; tgt_id                                                                                                   ;; target ID
                                                                                         ;; aux_id                                                                                                   ;; auxiliary ID
                                                                                         ;; src_offset_hi                                                                                            ;; source offset high
                                                                                         32                                                                                                       ;; source offset low
                                                                                         ;; tgt_offset_lo                                                                                            ;; target offset low
                                                                                         ;; size                                                                                                     ;; size
                                                                                         (precompile-processing---dup-cdo)                                                                        ;; reference offset
                                                                                         (precompile-processing---dup-cds)                                                                        ;; reference size
                                                                                         ;; success_bit                                                                                              ;; success bit
                                                                                         (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---ebs-analysis)            ;; limb 1          ;; TODO: remove SELF REFERENCE
                                                                                         (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---ebs-analysis)            ;; limb 2          ;; TODO: remove SELF REFERENCE
                                                                                         ;; exo_sum                                                                                                  ;; weighted exogenous module flag sum
                                                                                         ;; phase                                                                                                    ;; phase
                                                                                         )))

(defun    (precompile-processing---MODEXP---ebs-hi)    (*    (precompile-processing---MODEXP---extract-ebs)    (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---ebs-analysis)))
(defun    (precompile-processing---MODEXP---ebs-lo)    (*    (precompile-processing---MODEXP---extract-ebs)    (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---ebs-analysis)))

(defconstraint    precompile-processing---MODEXP---ebs-analysis---setting-OOB-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (set-OOB-instruction---modexp-xbs    precompile-processing---MODEXP---misc-row-offset---ebs-analysis         ;; offset
                                                       (precompile-processing---MODEXP---ebs-hi)                               ;; high part of some {b,e,m}bs
                                                       (precompile-processing---MODEXP---ebs-lo)                               ;; low  part of some {b,e,m}bs
                                                       0                                                                       ;; low  part of some {b,e,m}bs
                                                       0                                                                       ;; bit indicating whether to compute max(xbs, ybs) or not
                                                       ))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    mbs extraction and analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---mbs-analysis---setting-misc-module-flags    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)
                          (+    (*    MISC_WEIGHT_MMU    (precompile-processing---MODEXP---extract-mbs))
                                MISC_WEIGHT_OOB)
                          ))

(defconstraint    precompile-processing---MODEXP---mbs-analysis---setting-MMU-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)
                                  (set-MMU-instruction---right-padded-word-extraction    precompile-processing---MODEXP---misc-row-offset---mbs-analysis                                          ;; offset
                                                                                         CONTEXT_NUMBER                                                                                           ;; source ID
                                                                                         ;; tgt_id                                                                                                   ;; target ID
                                                                                         ;; aux_id                                                                                                   ;; auxiliary ID
                                                                                         ;; src_offset_hi                                                                                            ;; source offset high
                                                                                         64                                                                                                       ;; source offset low
                                                                                         ;; tgt_offset_lo                                                                                            ;; target offset low
                                                                                         ;; size                                                                                                     ;; size
                                                                                         (precompile-processing---dup-cdo)                                                                        ;; reference offset
                                                                                         (precompile-processing---dup-cds)                                                                        ;; reference size
                                                                                         ;; success_bit                                                                                              ;; success bit
                                                                                         (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)            ;; limb 1          ;; TODO: remove SELF REFERENCE
                                                                                         (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)            ;; limb 2          ;; TODO: remove SELF REFERENCE
                                                                                         ;; exo_sum                                                                                                  ;; weighted exogenous module flag sum
                                                                                         ;; phase                                                                                                    ;; phase
                                                                                         )))

(defun    (precompile-processing---MODEXP---mbs-hi)    (*    (precompile-processing---MODEXP---extract-mbs)    (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)))
(defun    (precompile-processing---MODEXP---mbs-lo)    (*    (precompile-processing---MODEXP---extract-mbs)    (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)))

(defconstraint    precompile-processing---MODEXP---mbs-analysis---setting-OOB-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (set-OOB-instruction---modexp-xbs    precompile-processing---MODEXP---misc-row-offset---mbs-analysis         ;; offset
                                                       (precompile-processing---MODEXP---mbs-hi)                               ;; high part of some {b,e,m}bs
                                                       (precompile-processing---MODEXP---mbs-lo)                               ;; low  part of some {b,e,m}bs
                                                       (precompile-processing---MODEXP---bbs-lo)                               ;; low  part of some {b,e,m}bs
                                                       1                                                                       ;; bit indicating whether to compute max(xbs, ybs) or not
                                                       ))


(defun    (precompile-processing---MODEXP---max-mbs-bbs)    (shift    [misc/OOB_DATA   7]    precompile-processing---MODEXP---misc-row-offset---mbs-analysis))
(defun    (precompile-processing---MODEXP---mbs-nonzero)    (shift    [misc/OOB_DATA   8]    precompile-processing---MODEXP---misc-row-offset---mbs-analysis)) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    leading_word analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---lead-log-analysis---setting-misc-module-flags    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)
                          (+    (*    MISC_WEIGHT_EXP    (precompile-processing---MODEXP---load-lead))
                                (*    MISC_WEIGHT_MMU    (precompile-processing---MODEXP---load-lead))
                                MISC_WEIGHT_OOB)
                          ))

(defconstraint    precompile-processing---MODEXP---lead-log-analysis---setting-OOB-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (set-OOB-instruction---modexp-lead    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis   ;; offset
                                                        (precompile-processing---MODEXP---bbs-lo)                                  ;; low part of bbs (base     byte size)
                                                        (precompile-processing---dup-cds)                                          ;; call data size
                                                        (precompile-processing---MODEXP---ebs-lo)                                  ;; low part of ebs (exponent byte size)
                                                        ))

(defun    (precompile-processing---MODEXP---load-lead)     (*    (precompile-processing---MODEXP---extract-bbs)    (shift    [misc/OOB_DATA  4]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))
(defun    (precompile-processing---MODEXP---cds-cutoff)    (*    (precompile-processing---MODEXP---extract-bbs)    (shift    [misc/OOB_DATA  6]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))
(defun    (precompile-processing---MODEXP---ebs-cutoff)    (*    (precompile-processing---MODEXP---extract-bbs)    (shift    [misc/OOB_DATA  7]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))
(defun    (precompile-processing---MODEXP---sub-ebs-32)    (*    (precompile-processing---MODEXP---extract-bbs)    (shift    [misc/OOB_DATA  8]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis))) ;; ""


(defconstraint    precompile-processing---MODEXP---lead-word-analysis---setting-MMU-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)
                                  (set-MMU-instruction---mload    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis                                   ;; offset
                                                                  CONTEXT_NUMBER                                                                                             ;; source ID
                                                                  ;; tgt_id                                                                                                     ;; target ID
                                                                  ;; aux_id                                                                                                     ;; auxiliary ID
                                                                  ;; src_offset_hi                                                                                              ;; source offset high
                                                                  (+    (precompile-processing---dup-cdo)
                                                                        96
                                                                        (precompile-processing---MODEXP---bbs-lo))                                                              ;; source offset low
                                                                  ;; tgt_offset_lo                                                                                              ;; target offset low
                                                                  ;; size                                                                                                       ;; size
                                                                  ;; ref_offset                                                                                                 ;; reference offset
                                                                  ;; ref_size                                                                                                   ;; reference size
                                                                  ;; success_bit                                                                                                ;; success bit
                                                                  (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)     ;; limb 1    ;; TODO: remove SELF REFERENCE
                                                                  (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)     ;; limb 2    ;; TODO: remove SELF REFERENCE
                                                                  ;; exo_sum                                                                                                    ;; weighted exogenous module flag sum
                                                                  ;; phase                                                                                                      ;; phase
                                                                  )))

(defun    (precompile-processing---MODEXP---raw-lead-hi)    (*    (precompile-processing---MODEXP---load-lead)    (shift    misc/MMU_LIMB_1    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))
(defun    (precompile-processing---MODEXP---raw-lead-lo)    (*    (precompile-processing---MODEXP---load-lead)    (shift    misc/MMU_LIMB_2    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)))


(defconstraint    precompile-processing---MODEXP---lead-word-analysis---setting-EXP-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (if-not-zero    (shift    misc/EXP_FLAG    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis)
                                  (set-EXP-instruction-MODEXP-lead-log
                                    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis          ;; row offset
                                    (precompile-processing---MODEXP---raw-lead-hi)                                    ;; raw leading word where exponent starts, high part
                                    (precompile-processing---MODEXP---raw-lead-lo)                                    ;; raw leading word where exponent starts, low  part
                                    (precompile-processing---MODEXP---cds-cutoff)                                     ;; min{max{cds - 96 - bbs, 0}, 32}
                                    (precompile-processing---MODEXP---ebs-cutoff)                                     ;; min{ebs, 32}
                                    )))

(defun    (precompile-processing---MODEXP---lead-log)           (*    (precompile-processing---MODEXP---load-lead)    (shift    [misc/EXP_DATA   5]    precompile-processing---MODEXP---misc-row-offset---leading-word-analysis))) ;; ""
(defun    (precompile-processing---MODEXP---modexp-full-log)    (+    (precompile-processing---MODEXP---lead-log)     (*    8    (precompile-processing---MODEXP---sub-ebs-32))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    pricing analysis row    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---pricing-analysis---setting-misc-module-flags    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---MODEXP---misc-row-offset---pricing)
                          MISC_WEIGHT_OOB
                          ))

(defconstraint    precompile-processing---MODEXP---pricing-analysis---setting-OOB-instruction    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (set-OOB-instruction---modexp-pricing    precompile-processing---MODEXP---misc-row-offset---pricing   ;; offset
                                                           (precompile-processing---dup-call-gas)                       ;; call gas i.e. gas provided to the precompile
                                                           (precompile-processing---dup-r@c)                            ;; return at capacity
                                                           (precompile-processing---MODEXP---modexp-full-log)           ;; leading (≤) word log of exponent
                                                           (precompile-processing---MODEXP---max-mbs-bbs)               ;; call data size
                                                           ))

(defun    (precompile-processing---MODEXP---ram-success)    (shift    [misc/OOB_DATA   4]    precompile-processing---MODEXP---misc-row-offset---pricing))
(defun    (precompile-processing---MODEXP---return-gas)     (shift    [misc/OOB_DATA   5]    precompile-processing---MODEXP---misc-row-offset---pricing))
(defun    (precompile-processing---MODEXP---r@c-nonzero)    (shift    [misc/OOB_DATA   8]    precompile-processing---MODEXP---misc-row-offset---pricing)) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;    Justifying precompile success / failure scenarios    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---MODEXP---justifying-success-failure-scenarios    (:guard    (precompile-processing---MODEXP---standard-precondition))
                  (begin
                    (eq!    (scenario-shorthand---PRC---success)        (precompile-processing---MODEXP---ram-success))
                    (eq!    (precompile-processing---prd-return-gas)    (precompile-processing---MODEXP---return-gas))
                    ))
