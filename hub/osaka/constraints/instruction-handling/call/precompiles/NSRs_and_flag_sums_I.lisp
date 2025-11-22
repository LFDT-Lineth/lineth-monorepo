(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.Y.Z.7 Flag sums and NSR's (I)    ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (precompile-processing---1st-half-NSR)
  (+    (*    CALL___first_half_nsr___prc_failure                (scenario-shorthand---PRC---failure))
        (*    CALL___first_half_nsr___prc_success_will_revert    scenario/PRC_SUCCESS_CALLER_WILL_REVERT)
        (*    CALL___first_half_nsr___prc_success_wont_revert    scenario/PRC_SUCCESS_CALLER_WONT_REVERT)
        ))
    
(defun        (precompile-processing---2nd-half-NSR)
  (+
    (*    (precompile-processing---2nd-half-NSR-for-ECRECOVER)             scenario/PRC_ECRECOVER)
    (*    (precompile-processing---2nd-half-NSR-for-SHA2-256)              scenario/PRC_SHA2-256)
    (*    (precompile-processing---2nd-half-NSR-for-RIPEMD-160)            scenario/PRC_RIPEMD-160)
    (*    (precompile-processing---2nd-half-NSR-for-IDENTITY)              scenario/PRC_IDENTITY)
    (*    (precompile-processing---2nd-half-NSR-for-MODEXP)                scenario/PRC_MODEXP)
    (*    (precompile-processing---2nd-half-NSR-for-ECADD)                 scenario/PRC_ECADD)
    (*    (precompile-processing---2nd-half-NSR-for-ECMUL)                 scenario/PRC_ECMUL)
    (*    (precompile-processing---2nd-half-NSR-for-ECPAIRING)             scenario/PRC_ECPAIRING)
    (*    (precompile-processing---2nd-half-NSR-for-BLAKE2f)               scenario/PRC_BLAKE2f)
    (*    (precompile-processing---2nd-half-NSR-for-all-BLS-precompiles)  (scenario-shorthand---PRC---common-BLS-address-bit-sum))
    (*    (precompile-processing---2nd-half-NSR-for-P256-VERIFY)           scenario/PRC_P256_VERIFY)
    ))

(defun        (precompile-processing---2nd-half-flag-sum)
  (+
    (*    (precompile-processing---2nd-half-flag-sum-for-ECRECOVER)             scenario/PRC_ECRECOVER)
    (*    (precompile-processing---2nd-half-flag-sum-for-SHA2-256)              scenario/PRC_SHA2-256)
    (*    (precompile-processing---2nd-half-flag-sum-for-RIPEMD-160)            scenario/PRC_RIPEMD-160)
    (*    (precompile-processing---2nd-half-flag-sum-for-IDENTITY)              scenario/PRC_IDENTITY)
    (*    (precompile-processing---2nd-half-flag-sum-for-MODEXP)                scenario/PRC_MODEXP)
    (*    (precompile-processing---2nd-half-flag-sum-for-ECADD)                 scenario/PRC_ECADD)
    (*    (precompile-processing---2nd-half-flag-sum-for-ECMUL)                 scenario/PRC_ECMUL)
    (*    (precompile-processing---2nd-half-flag-sum-for-ECPAIRING)             scenario/PRC_ECPAIRING)
    (*    (precompile-processing---2nd-half-flag-sum-for-BLAKE2f)               scenario/PRC_BLAKE2f)
    (*    (precompile-processing---2nd-half-flag-sum-for-all-BLS-precompiles)  (scenario-shorthand---PRC---common-BLS-address-bit-sum))
    (*    (precompile-processing---2nd-half-flag-sum-for-P256-VERIFY)           scenario/PRC_P256_VERIFY)
    ))

;; Stand failure / success shorthands
(defun    (precompile-processing---flag-sum-standard-success)
  (+      (shift    PEEK_AT_SCENARIO         0)
          (shift    PEEK_AT_MISCELLANEOUS    1)
          (shift    PEEK_AT_MISCELLANEOUS    2)
          (shift    PEEK_AT_MISCELLANEOUS    3)
          (shift    PEEK_AT_CONTEXT          4)
          ))
(defconst    precompile-processing---nsr-standard-success    5)

(defun    (precompile-processing---flag-sum-standard-failure)
  (+      (shift    PEEK_AT_SCENARIO         0)
          (shift    PEEK_AT_MISCELLANEOUS    1)
          (shift    PEEK_AT_CONTEXT          2)
          ))
(defconst    precompile-processing---nsr-standard-failure    3)

