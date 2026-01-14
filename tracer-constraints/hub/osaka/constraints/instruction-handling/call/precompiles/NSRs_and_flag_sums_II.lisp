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
;;    X.Y.Z.7 Flag sums and NSR's (II)    ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; precompile individualized shorthands


;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;    ECRECOVER    ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;


;; flag sum
(defun     (precompile-processing---2nd-half-flag-sum-for-ECRECOVER)
  (+  (*   (precompile-processing---flag-sum-ECRECOVER-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*   (precompile-processing---flag-sum-ECRECOVER-success)    (scenario-shorthand---PRC---success))
      ))
;; non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-ECRECOVER)
  (+  (*  (precompile-processing---nsr-ECRECOVER-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-ECRECOVER-success)    (scenario-shorthand---PRC---success))
      ))
;; flag sum shorthands
(defun    (precompile-processing---flag-sum-ECRECOVER-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECRECOVER-success)    (precompile-processing---flag-sum-standard-success))
;; non stack rows shorthands
(defun    (precompile-processing---nsr-ECRECOVER-FKTH)            precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECRECOVER-success)         precompile-processing---nsr-standard-success)
;; NB: the failure scenario FAILURE_KNOWN_TO_RAM is impossible


;;;;;;;;;;;;;;;;;;;;
;;                ;;
;;    SHA2-256    ;;
;;                ;;
;;;;;;;;;;;;;;;;;;;;


;; flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-SHA2-256)
  (+  (*  (precompile-processing---flag-sum-SHA2-256-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-SHA2-256-success)    (scenario-shorthand---PRC---success))
      ))
;; non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-SHA2-256)
  (+  (*  (precompile-processing---nsr-SHA2-256-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-SHA2-256-success)    (scenario-shorthand---PRC---success))
      ))
;; non stack rows shorthands
(defun    (precompile-processing---nsr-SHA2-256-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-SHA2-256-success)    precompile-processing---nsr-standard-success) ;; ""
;; flag sum shorthands
(defun    (precompile-processing---flag-sum-SHA2-256-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-SHA2-256-success)    (precompile-processing---flag-sum-standard-success))
;; NB: the failure scenario FAILURE_KNOWN_TO_RAM is impossible


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;    RIPEMD-160    ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;


;; RIPEMD-160 flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-RIPEMD-160)
  (+  (*  (precompile-processing---flag-sum-RIPEMD-160-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-RIPEMD-160-success)    (scenario-shorthand---PRC---success))
          ))
;; non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-RIPEMD-160)
  (+  (*  (precompile-processing---nsr-RIPEMD-160-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-RIPEMD-160-success)    (scenario-shorthand---PRC---success))
          ))
;; non stack rows shorthands
(defun    (precompile-processing---nsr-RIPEMD-160-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-RIPEMD-160-success)    precompile-processing---nsr-standard-success)
;; flag sum shorthands
(defun    (precompile-processing---flag-sum-RIPEMD-160-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-RIPEMD-160-success)    (precompile-processing---flag-sum-standard-success))
;; NB: the failure scenario FAILURE_KNOWN_TO_RAM is impossible


;;;;;;;;;;;;;;;;;;;;
;;                ;;
;;    IDENTITY    ;;
;;                ;;
;;;;;;;;;;;;;;;;;;;;


;; flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-IDENTITY)
  (+  (*  (precompile-processing---flag-sum-IDENTITY-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-IDENTITY-success)    (scenario-shorthand---PRC---success))
          ))
;; non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-IDENTITY)
  (+  (*   precompile-processing---nsr-IDENTITY-FKTH         scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*   precompile-processing---nsr-IDENTITY-success     (scenario-shorthand---PRC---success))
           ))
;; non stack rows shorthands
(defconst  precompile-processing---nsr-IDENTITY-FKTH        precompile-processing---nsr-standard-failure)
(defconst  precompile-processing---nsr-IDENTITY-success     4)
;; flag sum shorthands
(defun    (precompile-processing---flag-sum-IDENTITY-FKTH)    (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-IDENTITY-success)
  (+      (shift    PEEK_AT_SCENARIO         0)
          (shift    PEEK_AT_MISCELLANEOUS    1)
          (shift    PEEK_AT_MISCELLANEOUS    2)
          (shift    PEEK_AT_CONTEXT          3)
          ))
;; NB: the failure scenario FAILURE_KNOWN_TO_RAM is impossible


;;;;;;;;;;;;;;;;;;
;;              ;;
;;    MODEXP    ;;
;;              ;;
;;;;;;;;;;;;;;;;;;


;; MODEXP flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-MODEXP)
  (+  (*  (precompile-processing---flag-sum-MODEXP-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-MODEXP-success)    (scenario-shorthand---PRC---success))
      ))
;; MODEXP non stack rows
(defconst   precompile-processing---MODEXP-nsr-FKTR            8)
(defconst   precompile-processing---MODEXP-nsr-success        13)
(defun     (precompile-processing---2nd-half-NSR-for-MODEXP)
  (+  (*    precompile-processing---MODEXP-nsr-FKTR           scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*    precompile-processing---MODEXP-nsr-success        (scenario-shorthand---PRC---success))
      ))
;; MODEXP non stack rows shorthands
(defun    (precompile-processing---flag-sum-MODEXP-FKTR)       (+
                                                                 (shift    PEEK_AT_SCENARIO         0                                                                       )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---cds---row-offset              )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-bbs---offset          )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-ebs---row-offset      )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-mbs---row-offset      )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-raw-lead---row-offset )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---pricing---row-offset          )
                                                                 (shift    PEEK_AT_CONTEXT          precompile-processing---MODEXP-context-row---FKTR---row-offset          )
                                                                 ))
(defun    (precompile-processing---flag-sum-MODEXP-success)    (+
                                                                 (shift    PEEK_AT_SCENARIO         0                                                                             )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---cds---row-offset                    )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-bbs---offset                )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-ebs---row-offset            )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-mbs---row-offset            )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---extract-raw-lead---row-offset       )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---pricing---row-offset                )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---copy-inputs-base---row-offset       )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---copy-inputs-exponent---row-offset   )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---copy-inputs-modulus---row-offset    )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---full-result-transfer---row-offset   )
                                                                 (shift    PEEK_AT_MISCELLANEOUS    precompile-processing---MODEXP-misc-row---partial-result-copy---row-offset    )
                                                                 (shift    PEEK_AT_CONTEXT          precompile-processing---MODEXP-context-row---success---row-offset             )
                                                                 ))
;; NB: the failure scenario FAILURE_KNOWN_TO_HUB is impossible
(defconst
  precompile-processing---MODEXP-misc-row---cds---row-offset                      1
  precompile-processing---MODEXP-misc-row---extract-bbs---offset                  2
  precompile-processing---MODEXP-misc-row---extract-ebs---row-offset              3
  precompile-processing---MODEXP-misc-row---extract-mbs---row-offset              4
  precompile-processing---MODEXP-misc-row---extract-raw-lead---row-offset         5
  precompile-processing---MODEXP-misc-row---pricing---row-offset                  6
  precompile-processing---MODEXP-misc-row---copy-inputs-base---row-offset         7
  precompile-processing---MODEXP-misc-row---copy-inputs-exponent---row-offset     8
  precompile-processing---MODEXP-misc-row---copy-inputs-modulus---row-offset      9
  precompile-processing---MODEXP-misc-row---full-result-transfer---row-offset    10
  precompile-processing---MODEXP-misc-row---partial-result-copy---row-offset     11

  precompile-processing---MODEXP-context-row---FKTR---row-offset                  7
  precompile-processing---MODEXP-context-row---success---row-offset              12
  )


;;;;;;;;;;;;;;;;;
;;             ;;
;;    ECADD    ;;
;;             ;;
;;;;;;;;;;;;;;;;;


;; ECADD flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-ECADD)
  (+  (*  (precompile-processing---flag-sum-ECADD-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-ECADD-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-ECADD-success)    (scenario-shorthand---PRC---success))
          ))
;; ECADD non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-ECADD)
  (+  (*  (precompile-processing---nsr-ECADD-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-ECADD-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---nsr-ECADD-success)    (scenario-shorthand---PRC---success))
          ))
;; ECADD non stack rows shorthands
(defun    (precompile-processing---nsr-ECADD-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECADD-FKTR)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECADD-success)    precompile-processing---nsr-standard-success)
;; ECADD flag sum shorthands
(defun    (precompile-processing---flag-sum-ECADD-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECADD-FKTR)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECADD-success)    (precompile-processing---flag-sum-standard-success))


;;;;;;;;;;;;;;;;;
;;             ;;
;;    ECMUL    ;;
;;             ;;
;;;;;;;;;;;;;;;;;


;; ECMUL flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-ECMUL)
  (+  (*  (precompile-processing---flag-sum-ECMUL-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-ECMUL-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-ECMUL-success)    (scenario-shorthand---PRC---success))
          ))
;; ECMUL non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-ECMUL)
  (+  (*  (precompile-processing---nsr-ECMUL-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-ECMUL-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---nsr-ECMUL-success)    (scenario-shorthand---PRC---success))
          ))
;; ECMUL non stack rows shorthands
(defun    (precompile-processing---nsr-ECMUL-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECMUL-FKTR)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECMUL-success)    precompile-processing---nsr-standard-success) ;; ""
;; ECMUL flag sum shorthands
(defun    (precompile-processing---flag-sum-ECMUL-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECMUL-FKTR)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECMUL-success)    (precompile-processing---flag-sum-standard-success))


;;;;;;;;;;;;;;;;;;;;;
;;                 ;;
;;    ECPAIRING    ;;
;;                 ;;
;;;;;;;;;;;;;;;;;;;;;


;; ECPAIRING flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-ECPAIRING)
  (+  (*  (precompile-processing---flag-sum-ECPAIRING-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-ECPAIRING-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-ECPAIRING-success)    (scenario-shorthand---PRC---success))
          ))
;; ECPAIRING non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-ECPAIRING)
  (+  (*  (precompile-processing---nsr-ECPAIRING-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-ECPAIRING-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---nsr-ECPAIRING-success)    (scenario-shorthand---PRC---success))
          ))
;; ECPAIRING non stack rows shorthands
(defun    (precompile-processing---nsr-ECPAIRING-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECPAIRING-FKTR)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-ECPAIRING-success)    precompile-processing---nsr-standard-success) ;; ""
;; ECPAIRING flag sum shorthands
(defun    (precompile-processing---flag-sum-ECPAIRING-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECPAIRING-FKTR)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-ECPAIRING-success)    (precompile-processing---flag-sum-standard-success))


;;;;;;;;;;;;;;;;;;;
;;               ;;
;;    BLAKE2f    ;;
;;               ;;
;;;;;;;;;;;;;;;;;;;


;; BLAKE2f flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-BLAKE2f)
  (+  (*  (precompile-processing---flag-sum-BLAKE2f-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-BLAKE2f-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-BLAKE2f-success)    (scenario-shorthand---PRC---success))
          ))
;; BLAKE2f non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-BLAKE2f)
  (+  (*  precompile-processing---nsr-standard-failure     scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  precompile-processing---nsr-BLAKE2f-FKTR         scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  precompile-processing---nsr-BLAKE2f-success      (scenario-shorthand---PRC---success))
          ))
;; BLAKE2f non stack rows shorthands
(defconst
  precompile-processing---nsr-BLAKE2f-FKTR       4
  precompile-processing---nsr-BLAKE2f-success    6
  )
;; BLAKE2f flag sum shorthands
(defun    (precompile-processing---flag-sum-BLAKE2f-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-BLAKE2f-FKTR)       (+      (shift    PEEK_AT_SCENARIO         0)
                                                                        (shift    PEEK_AT_MISCELLANEOUS    1)
                                                                        (shift    PEEK_AT_MISCELLANEOUS    2)
                                                                        (shift    PEEK_AT_CONTEXT          3)
                                                                        ))
(defun    (precompile-processing---flag-sum-BLAKE2f-success)    (+      (shift    PEEK_AT_SCENARIO         0)
                                                                        (shift    PEEK_AT_MISCELLANEOUS    1)
                                                                        (shift    PEEK_AT_MISCELLANEOUS    2)
                                                                        (shift    PEEK_AT_MISCELLANEOUS    3)
                                                                        (shift    PEEK_AT_MISCELLANEOUS    4)
                                                                        (shift    PEEK_AT_CONTEXT          5)
                                                                        ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    All BLS precompiles    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; the full list of applicable precompiles is:
;;
;; - POINT_EVALUATION
;; - BLS_G1_ADD
;; - BLS_G1_MSM
;; - BLS_G2_ADD
;; - BLS_G2_MSM
;; - BLS_PAIRING_CHECK
;; - BLS_MAP_FP_TO_G1
;; - BLS_MAP_FP2_TO_G2


;; "all BLS" flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-all-BLS-precompiles)
  (+  (*  (precompile-processing---flag-sum-all-BLS-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-all-BLS-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-all-BLS-success)    (scenario-shorthand---PRC---success))
          ))
;; "all BLS" non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-all-BLS-precompiles)
  (+  (*  (precompile-processing---nsr-all-BLS-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-all-BLS-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---nsr-all-BLS-success)    (scenario-shorthand---PRC---success))
          ))
;; "all BLS" non stack rows shorthands
(defun    (precompile-processing---nsr-all-BLS-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-all-BLS-FKTR)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-all-BLS-success)    precompile-processing---nsr-standard-success) ;; ""
;; "all BLS" flag sum shorthands
(defun    (precompile-processing---flag-sum-all-BLS-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-all-BLS-FKTR)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-all-BLS-success)    (precompile-processing---flag-sum-standard-success))

;;;;;;;;;;;;;;;;;;;
;;               ;;
;;  P256_VERIFY  ;;
;;               ;;
;;;;;;;;;;;;;;;;;;;

;; P256_VERIFY flag sum
(defun    (precompile-processing---2nd-half-flag-sum-for-P256-VERIFY)
  (+  (*  (precompile-processing---flag-sum-P256-VERIFY-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---flag-sum-P256-VERIFY-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---flag-sum-P256-VERIFY-success)    (scenario-shorthand---PRC---success))
          ))
;; P256_VERIFY non stack rows
(defun    (precompile-processing---2nd-half-NSR-for-P256-VERIFY)
  (+  (*  (precompile-processing---nsr-P256-VERIFY-FKTH)       scenario/PRC_FAILURE_KNOWN_TO_HUB)
      (*  (precompile-processing---nsr-P256-VERIFY-FKTR)       scenario/PRC_FAILURE_KNOWN_TO_RAM)
      (*  (precompile-processing---nsr-P256-VERIFY-success)    (scenario-shorthand---PRC---success))
          ))
;; P256_VERIFY non stack rows shorthands
(defun    (precompile-processing---nsr-P256-VERIFY-FKTH)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-P256-VERIFY-FKTR)       precompile-processing---nsr-standard-failure)
(defun    (precompile-processing---nsr-P256-VERIFY-success)    precompile-processing---nsr-standard-success)
;; P256_VERIFY flag sum shorthands
(defun    (precompile-processing---flag-sum-P256-VERIFY-FKTH)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-P256-VERIFY-FKTR)       (precompile-processing---flag-sum-standard-failure))
(defun    (precompile-processing---flag-sum-P256-VERIFY-success)    (precompile-processing---flag-sum-standard-success))

;; NOTE: while defined here, FKTR ≡ impossible for the P256_VERIFY precompile
