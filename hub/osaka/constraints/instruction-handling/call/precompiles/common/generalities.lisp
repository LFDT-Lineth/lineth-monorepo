(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.Y.Z.1 Introduction      ;;
;;    X.Y.Z.2 Representation    ;;
;;    X.Y.Z.3 Generalities      ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---common---precondition)    (force-bin   (*    PEEK_AT_SCENARIO
                                                                                (scenario-shorthand---PRC---common-address-bit-sum))))


(defconstraint    precompile-processing---common---setting-MISC-module-flags
                  (:guard     (precompile-processing---common---precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    (weighted-MISC-flag-sum-sans-MMU    1)    MISC_WEIGHT_OOB)
                    (eq!    (shift    misc/MMU_FLAG             1)    (precompile-processing---common---OOB-extract-call-data))
                    ))

(defconstraint    precompile-processing---common---setting-OOB-instruction
                  (:guard    (precompile-processing---common---precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (set-OOB-instruction---common    precompile-processing---common---1st-misc-row---row-offset  ;; offset
                                                   (precompile-processing---common---OOB-instruction)          ;; relevant OOB instruction
                                                   (precompile-processing---dup-callee-gas)                    ;; call gas i.e. gas provided to the precompile
                                                   (precompile-processing---dup-cds)                           ;; call data size
                                                   (precompile-processing---dup-r@c)                           ;; return at capacity
                                                   )
                  )

(defun    (precompile-processing---common---OOB-instruction)
  (+    (*    OOB_INST_ECRECOVER           scenario/PRC_ECRECOVER          )
        (*    OOB_INST_SHA2                scenario/PRC_SHA2-256           )
        (*    OOB_INST_RIPEMD              scenario/PRC_RIPEMD-160         )
        (*    OOB_INST_IDENTITY            scenario/PRC_IDENTITY           )
        (*    OOB_INST_ECADD               scenario/PRC_ECADD              )
        (*    OOB_INST_ECMUL               scenario/PRC_ECMUL              )
        (*    OOB_INST_ECPAIRING           scenario/PRC_ECPAIRING          )
        (*    OOB_INST_POINT_EVALUATION    scenario/PRC_POINT_EVALUATION   )
        (*    OOB_INST_BLS_G1_ADD          scenario/PRC_BLS_G1_ADD         )
        (*    OOB_INST_BLS_G1_MSM          scenario/PRC_BLS_G1_MSM         )
        (*    OOB_INST_BLS_G2_ADD          scenario/PRC_BLS_G2_ADD         )
        (*    OOB_INST_BLS_G2_MSM          scenario/PRC_BLS_G2_MSM         )
        (*    OOB_INST_BLS_PAIRING_CHECK   scenario/PRC_BLS_PAIRING_CHECK  )
        (*    OOB_INST_BLS_MAP_FP_TO_G1    scenario/PRC_BLS_MAP_FP_TO_G1   )
        (*    OOB_INST_BLS_MAP_FP2_TO_G2   scenario/PRC_BLS_MAP_FP2_TO_G2  )
        (*    OOB_INST_P256_VERIFY         scenario/PRC_P256_VERIFY        )
        ))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; OOB related shorthands ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (precompile-processing---common---OOB-hub-success)          (shift    [misc/OOB_DATA    4]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-return-gas)           (shift    [misc/OOB_DATA    5]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-extract-call-data)    (shift    [misc/OOB_DATA    6]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-empty-call-data)      (shift    [misc/OOB_DATA    7]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-r@c-nonzero)          (shift    [misc/OOB_DATA    8]    precompile-processing---common---1st-misc-row---row-offset)) ;; ""


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; OOB shorthands formal properties and sanity checks ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defproperty      precompile-processing---common---sanity-checks---binarities
                  (if-not-zero   (precompile-processing---common---precondition)
                                 (begin
                                   (is-binary   (precompile-processing---common---OOB-hub-success)       )
                                   (is-binary   (precompile-processing---common---OOB-extract-call-data) )
                                   (is-binary   (precompile-processing---common---OOB-empty-call-data)   )
                                   (is-binary   (precompile-processing---common---OOB-r@c-nonzero)       )
                                   )))

(defproperty      precompile-processing---common---sanity-checks---unconditional-extract-empty-exclusivity
                  (if-not-zero   (precompile-processing---common---precondition)
                                 (vanishes!   (*  (precompile-processing---common---OOB-extract-call-data)
                                                  (precompile-processing---common---OOB-empty-call-data)
                                                  ))))

(defproperty      precompile-processing---common---sanity-checks---the-empty-call-data-bit-must-vanish-starting-with-Cancun
                  (if-not-zero   (precompile-processing---common---precondition)
                                 (if-not-zero   (+   (scenario-shorthand---PRC---common-Cancun-address-bit-sum)
                                                     (scenario-shorthand---PRC---common-Prague-address-bit-sum)
                                                     (scenario-shorthand---PRC---common-Osaka-address-bit-sum))
                                                (vanishes!   (precompile-processing---common---OOB-empty-call-data))
                                                )))

(defproperty      precompile-processing---common---sanity-checks---neat-splitting-of-hub-success-bit-prior-to-Osaka
                  (if-not-zero   (precompile-processing---common---precondition)
                                 (if-not-zero   (+   (scenario-shorthand---PRC---common-London-address-bit-sum)
                                                     (scenario-shorthand---PRC---common-Cancun-address-bit-sum)
                                                     (scenario-shorthand---PRC---common-Prague-address-bit-sum)
                                                     )
                                                (eq!   (precompile-processing---common---OOB-hub-success)
                                                       (+   (precompile-processing---common---OOB-extract-call-data)
                                                            (precompile-processing---common---OOB-empty-call-data)
                                                            )))))

;; we verify empirically that 0 ≤ extract_call_data ≤ hub_success (≤ 1)
(defproperty      precompile-processing---common---sanity-checks---not-so-neat-splitting-of-hub-success-for-Osaka
                  (if-not-zero   (precompile-processing---common---precondition)
                                 (if-not-zero   (scenario-shorthand---PRC---common-Osaka-address-bit-sum)
                                                (if-not-zero    (precompile-processing---common---OOB-extract-call-data)
                                                                (eq!    (precompile-processing---common---OOB-hub-success)
                                                                        1
                                                                        )))))

(defconstraint    precompile-processing---common---setting-MMU-instruction
                  (:guard    (precompile-processing---common---precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---common---1st-misc-row---row-offset)
                                  (begin
                                    (if-not-zero    scenario/PRC_IDENTITY
                                                    (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---common---1st-misc-row---row-offset   ;; offset
                                                                                                      CONTEXT_NUMBER                                               ;; source ID
                                                                                                      (+    1    HUB_STAMP)                                        ;; target ID
                                                                                                      ;; aux_id                                                    ;; auxiliary ID
                                                                                                      ;; src_offset_hi                                             ;; source offset high
                                                                                                      (precompile-processing---dup-cdo)                            ;; source offset low
                                                                                                      ;; tgt_offset_lo                                             ;; target offset low
                                                                                                      (precompile-processing---dup-cds)                            ;; size
                                                                                                      0                                                            ;; reference offset
                                                                                                      (precompile-processing---dup-cds)                            ;; reference size
                                                                                                      ;; success_bit                                               ;; success bit
                                                                                                      ;; limb_1                                                    ;; limb 1
                                                                                                      ;; limb_2                                                    ;; limb 2
                                                                                                      ;; exo_sum                                                   ;; weighted exogenous module flag sum
                                                                                                      ;; phase                                                     ;; phase
                                                                                                      ))
                                    (if-not-zero    (scenario-shorthand---PRC---common-except-identity-address-bit-sum)
                                                    (set-MMU-instruction---ram-to-exo-with-padding    precompile-processing---common---1st-misc-row---row-offset   ;; offset
                                                                                                      CONTEXT_NUMBER                                               ;; source ID
                                                                                                      (+    1    HUB_STAMP)                                        ;; target ID
                                                                                                      0                                                            ;; auxiliary ID (here: ∅)
                                                                                                      ;; src_offset_hi                                                ;; source offset high
                                                                                                      (precompile-processing---dup-cdo)                            ;; source offset low
                                                                                                      ;; tgt_offset_lo                                                ;; target offset low
                                                                                                      (precompile-processing---dup-cds)                            ;; size
                                                                                                      ;; ref_offset                                                   ;; reference offset
                                                                                                      (precompile-processing---common---MMU-reference-size)        ;; reference size
                                                                                                      (precompile-processing---common---MMU-success-bit)           ;; success bit (TODO: ugly self referential constraint ...)
                                                                                                      ;; limb_1                                                       ;; limb 1
                                                                                                      ;; limb_2                                                       ;; limb 2
                                                                                                      (precompile-processing---common---MMU-exo-sum)               ;; weighted exogenous module flag sum
                                                                                                      (precompile-processing---common---MMU-phase)                 ;; phase
                                                                                                      ))
                                    )
                                  ))

(defun    (precompile-processing---common---MMU-success-bit)
  (shift    misc/MMU_SUCCESS_BIT    precompile-processing---common---1st-misc-row---row-offset))

(defun    (precompile-processing---common---MMU-reference-size)
  (+    (*   128                                 scenario/PRC_ECRECOVER                                    )
        (*   (precompile-processing---dup-cds)   scenario/PRC_SHA2-256                                     )
        (*   (precompile-processing---dup-cds)   scenario/PRC_RIPEMD-160                                   )
        (*   128                                 scenario/PRC_ECADD                                        )
        (*   96                                  scenario/PRC_ECMUL                                        )
        (*   (precompile-processing---dup-cds)   scenario/PRC_ECPAIRING                                    )
        (*   (precompile-processing---dup-cds)   (scenario-shorthand---PRC---common-BLS-address-bit-sum)   )
        (*   (precompile-processing---dup-cds)   scenario/PRC_P256_VERIFY                                  )
        ))

(defun    (precompile-processing---common---MMU-exo-sum)
  (+    (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECRECOVER                                 )
        (*    EXO_SUM_WEIGHT_RIPSHA    scenario/PRC_SHA2-256                                  )
        (*    EXO_SUM_WEIGHT_RIPSHA    scenario/PRC_RIPEMD-160                                )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECADD                                     )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECMUL                                     )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECPAIRING                                 )
        (*    EXO_SUM_WEIGHT_BLSDATA  (scenario-shorthand---PRC---common-BLS-address-bit-sum) )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_P256_VERIFY                               )
        ))

(defun    (precompile-processing---common---MMU-phase)
  (+    (*    PHASE_ECRECOVER_DATA             scenario/PRC_ECRECOVER         )
        (*    PHASE_SHA2_DATA                  scenario/PRC_SHA2-256          )
        (*    PHASE_RIPEMD_DATA                scenario/PRC_RIPEMD-160        )
        (*    PHASE_ECADD_DATA                 scenario/PRC_ECADD             )
        (*    PHASE_ECMUL_DATA                 scenario/PRC_ECMUL             )
        (*    PHASE_ECPAIRING_DATA             scenario/PRC_ECPAIRING         )
        (*    PHASE_POINT_EVALUATION_DATA      scenario/PRC_POINT_EVALUATION  )
        (*    PHASE_BLS_G1_ADD_DATA            scenario/PRC_BLS_G1_ADD        )
        (*    PHASE_BLS_G1_MSM_DATA            scenario/PRC_BLS_G1_MSM        )
        (*    PHASE_BLS_G2_ADD_DATA            scenario/PRC_BLS_G2_ADD        )
        (*    PHASE_BLS_G2_MSM_DATA            scenario/PRC_BLS_G2_MSM        )
        (*    PHASE_BLS_PAIRING_CHECK_DATA     scenario/PRC_BLS_PAIRING_CHECK )
        (*    PHASE_BLS_MAP_FP_TO_G1_DATA      scenario/PRC_BLS_MAP_FP_TO_G1  )
        (*    PHASE_BLS_MAP_FP2_TO_G2_DATA     scenario/PRC_BLS_MAP_FP2_TO_G2 )
        (*    PHASE_P256_VERIFY_DATA           scenario/PRC_P256_VERIFY       )
        ))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECRECOVER related shorthands ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defun    (precompile-processing---common---address-recovery-failure)    (+      (precompile-processing---common---OOB-empty-call-data)
                                                                                 (*    (precompile-processing---common---OOB-extract-call-data)
                                                                                       (-    1    (precompile-processing---common---MMU-success-bit)))
                                                                                 ))
(defun    (precompile-processing---common---address-recovery-success)    (*      (precompile-processing---common---OOB-extract-call-data)
                                                                                 (precompile-processing---common---MMU-success-bit)
                                                                                 ))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECRECOVER related shorthands sanity checks ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defproperty    precompile-processing---common---address-recovery-success-shorthands-sanity-checks
                (if-not-zero   (precompile-processing---common---precondition)
                               (if-not-zero    scenario/PRC_ECRECOVER
                                               (begin
                                                 (is-binary   (precompile-processing---common---address-recovery-failure))
                                                 (is-binary   (precompile-processing---common---address-recovery-success))
                                                 (eq!         (precompile-processing---common---OOB-hub-success)
                                                              (+ (precompile-processing---common---address-recovery-failure)
                                                                 (precompile-processing---common---address-recovery-success)
                                                                 ))))))


;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECADD, ECMUL, ECPAIRING and BLS related shorthands ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defun    (precompile-processing---common---malformed-data)     (*    (precompile-processing---common---OOB-extract-call-data)
                                                                      (-   1    (precompile-processing---common---MMU-success-bit))))
(defun    (precompile-processing---common---wellformed-data)    (+    (precompile-processing---common---OOB-empty-call-data)
                                                                      (*    (precompile-processing---common---OOB-extract-call-data)
                                                                            (precompile-processing---common---MMU-success-bit))))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECADD, ECMUL, ECPAIRING and BLS related shorthands sanity checks ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defun    (common-precompiles-where-wellformed-and-malformed-data-matters)   (+   scenario/PRC_ECADD
                                                                                  scenario/PRC_ECMUL
                                                                                  scenario/PRC_ECPAIRING
                                                                                  (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                                                                  ))


(defproperty    precompile-processing---common---well-and-mal-formed-data-shorthands-sanity-checks
                (if-not-zero   (precompile-processing---common---precondition)
                               (if-not-zero    (common-precompiles-where-wellformed-and-malformed-data-matters)
                                               (begin
                                                 (is-binary   (precompile-processing---common---malformed-data))
                                                 (is-binary   (precompile-processing---common---wellformed-data))
                                                 (eq!         (precompile-processing---common---OOB-hub-success)
                                                              (+   (precompile-processing---common---malformed-data)
                                                                   (precompile-processing---common---wellformed-data)
                                                                   ))))))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; P256_VERIFY related shorthands ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defun    (precompile-processing---common---p256-sufficient-gas-wrong-cds)    (-   (precompile-processing---common---OOB-hub-success)
                                                                                   (precompile-processing---common---OOB-extract-call-data)
                                                                                   ))
(defun    (precompile-processing---common---p256-sig-verification-failure)    (*   (precompile-processing---common---OOB-extract-call-data)
                                                                                   (-    1    (precompile-processing---common---MMU-success-bit))
                                                                                   ))
(defun    (precompile-processing---common---p256-sig-verification-success)    (*   (precompile-processing---common---OOB-extract-call-data)
                                                                                   (precompile-processing---common---MMU-success-bit)
                                                                                   ))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; P256_VERIFY related shorthands sanity checks ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defproperty    precompile-processing---common---signature-verification-success-shorthands-sanity-checks
                (if-not-zero   (precompile-processing---common---precondition)
                               (if-not-zero    scenario/PRC_P256_VERIFY
                                               (begin
                                                 (is-binary   (precompile-processing---common---p256-sufficient-gas-wrong-cds))
                                                 (is-binary   (precompile-processing---common---p256-sig-verification-failure))
                                                 (is-binary   (precompile-processing---common---p256-sig-verification-success))
                                                 (eq!         (precompile-processing---common---OOB-hub-success)
                                                              (+ (precompile-processing---common---p256-sufficient-gas-wrong-cds)
                                                                 (precompile-processing---common---p256-sig-verification-failure)
                                                                 (precompile-processing---common---p256-sig-verification-success)
                                                                 ))))))



(defconstraint    precompile-processing---common---justifying-success-scenario
                  (:guard    (precompile-processing---common---precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (scenario-shorthand---PRC---success)
                          (+    (*    (precompile-processing---common---OOB-hub-success)
                                      (+    scenario/PRC_ECRECOVER
                                            scenario/PRC_SHA2-256
                                            scenario/PRC_RIPEMD-160
                                            scenario/PRC_IDENTITY
                                            scenario/PRC_P256_VERIFY
                                            ))
                                (*    (precompile-processing---common---wellformed-data)
                                      (+    scenario/PRC_ECADD
                                            scenario/PRC_ECMUL
                                            scenario/PRC_ECPAIRING
                                            (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                            ))
                                )))

(defconstraint    precompile-processing---common---justifying-FAILURE_KNOWN_TO_HUB    (:guard    (precompile-processing---common---precondition))
                  (eq!    scenario/PRC_FAILURE_KNOWN_TO_HUB
                          (-    1    (precompile-processing---common---OOB-hub-success))))

(defconstraint    precompile-processing---common---justifying-FAILURE_KNOWN_TO_RAM    (:guard    (precompile-processing---common---precondition))
                  (eq!    scenario/PRC_FAILURE_KNOWN_TO_RAM
                          (*    (precompile-processing---common---malformed-data)
                                (+    scenario/PRC_ECADD
                                      scenario/PRC_ECMUL
                                      scenario/PRC_ECPAIRING
                                      (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                      ))))

(defconstraint    precompile-processing---common---justifying-return-gas-prediction    (:guard    (precompile-processing---common---precondition))
                  (begin
                    (if-not-zero    (scenario-shorthand---PRC---failure)
                                    (vanishes!    (precompile-processing---prd-return-gas)))
                    (if-not-zero    (scenario-shorthand---PRC---success)
                                    (eq!          (precompile-processing---prd-return-gas)
                                                  (precompile-processing---common---OOB-return-gas)))
                    ))
