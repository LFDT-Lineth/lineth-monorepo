(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   10.5 SCEN/PRC instruction shorthands   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (scenario-shorthand---PRC---common-London-address-bit-sum)
  (force-bin   (+   scenario/PRC_ECRECOVER
                    scenario/PRC_SHA2-256
                    scenario/PRC_RIPEMD-160
                    scenario/PRC_IDENTITY
                    ;; scenario/PRC_MODEXP
                    scenario/PRC_ECADD
                    scenario/PRC_ECMUL
                    scenario/PRC_ECPAIRING
                    ;; scenario/PRC_BLAKE2f
                    ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                    ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                    ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                    ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                    ;; scenario/PRC_POINT_EVALUATION
                    ;; scenario/PRC_BLS_G1_ADD
                    ;; scenario/PRC_BLS_G1_MSM
                    ;; scenario/PRC_BLS_G2_ADD
                    ;; scenario/PRC_BLS_G2_MSM
                    ;; scenario/PRC_BLS_PAIRING_CHECK
                    ;; scenario/PRC_BLS_MAP_FP_TO_G1
                    ;; scenario/PRC_BLS_MAP_FP2_TO_G2
                    ;; (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                    ;; scenario/PRC_P256_VERIFY
                    )))


(defun (scenario-shorthand---PRC---common-Cancun-address-bit-sum)
  (force-bin   (+   ;; scenario/PRC_ECRECOVER
                 ;; scenario/PRC_SHA2-256
                 ;; scenario/PRC_RIPEMD-160
                 ;; scenario/PRC_IDENTITY
                 ;; scenario/PRC_MODEXP
                 ;; scenario/PRC_ECADD
                 ;; scenario/PRC_ECMUL
                 ;; scenario/PRC_ECPAIRING
                 ;; scenario/PRC_BLAKE2f
                 ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                 ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                 ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                 ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                 scenario/PRC_POINT_EVALUATION
                 ;; scenario/PRC_BLS_G1_ADD
                 ;; scenario/PRC_BLS_G1_MSM
                 ;; scenario/PRC_BLS_G2_ADD
                 ;; scenario/PRC_BLS_G2_MSM
                 ;; scenario/PRC_BLS_PAIRING_CHECK
                 ;; scenario/PRC_BLS_MAP_FP_TO_G1
                 ;; scenario/PRC_BLS_MAP_FP2_TO_G2
                 ;; (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                 ;; scenario/PRC_P256_VERIFY
                 )))


(defun (scenario-shorthand---PRC---common-Prague-address-bit-sum)
  (force-bin   (+   ;; scenario/PRC_ECRECOVER
                 ;; scenario/PRC_SHA2-256
                 ;; scenario/PRC_RIPEMD-160
                 ;; scenario/PRC_IDENTITY
                 ;; scenario/PRC_MODEXP
                 ;; scenario/PRC_ECADD
                 ;; scenario/PRC_ECMUL
                 ;; scenario/PRC_ECPAIRING
                 ;; scenario/PRC_BLAKE2f
                 ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                 ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                 ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                 ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                 ;; scenario/PRC_POINT_EVALUATION
                 scenario/PRC_BLS_G1_ADD
                 scenario/PRC_BLS_G1_MSM
                 scenario/PRC_BLS_G2_ADD
                 scenario/PRC_BLS_G2_MSM
                 scenario/PRC_BLS_PAIRING_CHECK
                 scenario/PRC_BLS_MAP_FP_TO_G1
                 scenario/PRC_BLS_MAP_FP2_TO_G2
                 ;; scenario/PRC_P256_VERIFY
                 )))


(defun (scenario-shorthand---PRC---common-Osaka-address-bit-sum)
  (force-bin   (+   ;; scenario/PRC_ECRECOVER
                 ;; scenario/PRC_SHA2-256
                 ;; scenario/PRC_RIPEMD-160
                 ;; scenario/PRC_IDENTITY
                 ;; scenario/PRC_MODEXP
                 ;; scenario/PRC_ECADD
                 ;; scenario/PRC_ECMUL
                 ;; scenario/PRC_ECPAIRING
                 ;; scenario/PRC_BLAKE2f
                 ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                 ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                 ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                 ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                 ;; scenario/PRC_POINT_EVALUATION
                 ;; scenario/PRC_BLS_G1_ADD
                 ;; scenario/PRC_BLS_G1_MSM
                 ;; scenario/PRC_BLS_G2_ADD
                 ;; scenario/PRC_BLS_G2_MSM
                 ;; scenario/PRC_BLS_PAIRING_CHECK
                 ;; scenario/PRC_BLS_MAP_FP_TO_G1
                 ;; scenario/PRC_BLS_MAP_FP2_TO_G2
                 scenario/PRC_P256_VERIFY
                 )))

;;  PRC/sum
(defun (scenario-shorthand---PRC---common-BLS-address-bit-sum)
  (force-bin   (+   (scenario-shorthand---PRC---common-Cancun-address-bit-sum)
                    (scenario-shorthand---PRC---common-Prague-address-bit-sum)
                    )))


;;  PRC/sum
(defun (scenario-shorthand---PRC---common-except-identity-address-bit-sum)
  (force-bin   (+   scenario/PRC_ECRECOVER
                    scenario/PRC_SHA2-256
                    scenario/PRC_RIPEMD-160
                    ;; scenario/PRC_IDENTITY
                    ;; scenario/PRC_MODEXP
                    scenario/PRC_ECADD
                    scenario/PRC_ECMUL
                    scenario/PRC_ECPAIRING
                    ;; scenario/PRC_BLAKE2f
                    ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                    ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                    ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                    ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                    ;; scenario/PRC_POINT_EVALUATION
                    ;; scenario/PRC_BLS_G1_ADD
                    ;; scenario/PRC_BLS_G1_MSM
                    ;; scenario/PRC_BLS_G2_ADD
                    ;; scenario/PRC_BLS_G2_MSM
                    ;; scenario/PRC_BLS_PAIRING_CHECK
                    ;; scenario/PRC_BLS_MAP_FP_TO_G1
                    ;; scenario/PRC_BLS_MAP_FP2_TO_G2
                    (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                    scenario/PRC_P256_VERIFY
                    )))


(defun (scenario-shorthand---PRC---common-address-bit-sum)
  (force-bin   (+  (scenario-shorthand---PRC---common-except-identity-address-bit-sum)
                   scenario/PRC_IDENTITY
                   )))


(defproperty    scenario-shorthands---the-two-definitions-of-common-prc-address-bit-sums-must-coincide
                (if-not-zero   PEEK_AT_SCENARIO
                               (eq!    (scenario-shorthand---PRC---common-address-bit-sum)
                                       (+   (scenario-shorthand---PRC---common-London-address-bit-sum)
                                            (scenario-shorthand---PRC---common-Cancun-address-bit-sum)
                                            (scenario-shorthand---PRC---common-Prague-address-bit-sum)
                                            (scenario-shorthand---PRC---common-Osaka-address-bit-sum)
                                            ))))

;;  PRC/sum
(defun (scenario-shorthand---PRC---full-address-bit-sum)
  (force-bin   (+   (scenario-shorthand---PRC---common-address-bit-sum)
                    scenario/PRC_MODEXP
                    scenario/PRC_BLAKE2f
                    )))

;;  PRC/may_only_fail_in_HUB
(defun (scenario-shorthand---PRC---may-only-fail-in-the-HUB)
  (force-bin   (+   scenario/PRC_ECRECOVER
                    scenario/PRC_SHA2-256
                    scenario/PRC_RIPEMD-160
                    scenario/PRC_IDENTITY
                    ;; scenario/PRC_MODEXP
                    ;; scenario/PRC_ECADD
                    ;; scenario/PRC_ECMUL
                    ;; scenario/PRC_ECPAIRING
                    ;; scenario/PRC_BLAKE2f
                    ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                    ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                    ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                    ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                    ;; scenario/PRC_POINT_EVALUATION
                    ;; scenario/PRC_BLS_G1_ADD
                    ;; scenario/PRC_BLS_G1_MSM
                    ;; scenario/PRC_BLS_G2_ADD
                    ;; scenario/PRC_BLS_G2_MSM
                    ;; scenario/PRC_BLS_PAIRING_CHECK
                    ;; scenario/PRC_BLS_MAP_FP_TO_G1
                    ;; scenario/PRC_BLS_MAP_FP2_TO_G2
                    scenario/PRC_P256_VERIFY
                    ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                    ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                    ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                    ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                    )))

;;  PRC/may_only_fail_in_RAM
(defun (scenario-shorthand---PRC---may-only-fail-in-the-RAM)
  (force-bin   (+
                 ;; scenario/PRC_ECRECOVER
                 ;; scenario/PRC_SHA2-256
                 ;; scenario/PRC_RIPEMD-160
                 ;; scenario/PRC_IDENTITY
                 scenario/PRC_MODEXP
                 ;; scenario/PRC_ECADD
                 ;; scenario/PRC_ECMUL
                 ;; scenario/PRC_ECPAIRING
                 ;; scenario/PRC_BLAKE2f
                 ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                 ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                 ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                 ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                 )))

;;  PRC/weighted_sum
(defun (scenario-shorthand---PRC---weighted-address-bit-sum)
  (+
    (*     1   scenario/PRC_ECRECOVER          )
    (*     2   scenario/PRC_SHA2-256           )
    (*     3   scenario/PRC_RIPEMD-160         )
    (*     4   scenario/PRC_IDENTITY           )
    (*     5   scenario/PRC_MODEXP             )
    (*     6   scenario/PRC_ECADD              )
    (*     7   scenario/PRC_ECMUL              )
    (*     8   scenario/PRC_ECPAIRING          )
    (*     9   scenario/PRC_BLAKE2f            )
    (*    10   scenario/PRC_POINT_EVALUATION   )
    (*    11   scenario/PRC_BLS_G1_ADD         )
    (*    12   scenario/PRC_BLS_G1_MSM         )
    (*    13   scenario/PRC_BLS_G2_ADD         )
    (*    14   scenario/PRC_BLS_G2_MSM         )
    (*    15   scenario/PRC_BLS_PAIRING_CHECK  )
    (*    16   scenario/PRC_BLS_MAP_FP_TO_G1   )
    (*    17   scenario/PRC_BLS_MAP_FP2_TO_G2  )
    (*   256   scenario/PRC_P256_VERIFY        )
    ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
    ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
    ))

;;  PRC/failure
(defun (scenario-shorthand---PRC---failure)
  (force-bin   (+
                 ;; scenario/PRC_ECRECOVER
                 ;; scenario/PRC_SHA2-256
                 ;; scenario/PRC_RIPEMD-160
                 ;; scenario/PRC_IDENTITY
                 ;; scenario/PRC_MODEXP
                 ;; scenario/PRC_ECADD
                 ;; scenario/PRC_ECMUL
                 ;; scenario/PRC_ECPAIRING
                 ;; scenario/PRC_BLAKE2f
                 ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                 ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                 scenario/PRC_FAILURE_KNOWN_TO_HUB
                 scenario/PRC_FAILURE_KNOWN_TO_RAM
                 )))

;;  PRC/success
(defun (scenario-shorthand---PRC---success)
  (force-bin   (+
                 ;; scenario/PRC_ECRECOVER
                 ;; scenario/PRC_SHA2-256
                 ;; scenario/PRC_RIPEMD-160
                 ;; scenario/PRC_IDENTITY
                 ;; scenario/PRC_MODEXP
                 ;; scenario/PRC_ECADD
                 ;; scenario/PRC_ECMUL
                 ;; scenario/PRC_ECPAIRING
                 ;; scenario/PRC_BLAKE2f
                 scenario/PRC_SUCCESS_CALLER_WILL_REVERT
                 scenario/PRC_SUCCESS_CALLER_WONT_REVERT
                 ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
                 ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
                 )))

;;  PRC/scenario_sum
(defun (scenario-shorthand---PRC---sum)
  (force-bin   (+   (scenario-shorthand---PRC---failure)
                    (scenario-shorthand---PRC---success)
                    )))

;; ;;  PRC/
;; (defun (scenario-shorthand---PRC---)
;;   (+
;;     scenario/PRC_ECRECOVER
;;     scenario/PRC_SHA2-256
;;     scenario/PRC_RIPEMD-160
;;     scenario/PRC_IDENTITY
;;     scenario/PRC_MODEXP
;;     scenario/PRC_ECADD
;;     scenario/PRC_ECMUL
;;     scenario/PRC_ECPAIRING
;;     scenario/PRC_BLAKE2f
;;     scenario/PRC_SUCCESS_CALLER_WILL_REVERT
;;     scenario/PRC_SUCCESS_CALLER_WONT_REVERT
;;     scenario/PRC_FAILURE_KNOWN_TO_HUB
;;     scenario/PRC_FAILURE_KNOWN_TO_RAM
;;     ))
