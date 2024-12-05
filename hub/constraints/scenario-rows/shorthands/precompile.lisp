(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   10.5 SCEN/PRC instruction shorthands   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;  PRC/sum
(defun (scenario-shorthand---PRC---common-except-identity-address-bit-sum)
  (+
    scenario/PRC_ECRECOVER
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
    ))

(defun (scenario-shorthand---PRC---common-address-bit-sum)
  (+  (scenario-shorthand---PRC---common-except-identity-address-bit-sum)
      scenario/PRC_IDENTITY
      ))

;;  PRC/sum
(defun (scenario-shorthand---PRC---full-address-bit-sum)
  (+  (scenario-shorthand---PRC---common-address-bit-sum)
      scenario/PRC_MODEXP
      scenario/PRC_BLAKE2f
      ))

;;  PRC/may_only_fail_in_HUB
(defun (scenario-shorthand---PRC---may-only-fail-in-the-HUB)
  (+
    scenario/PRC_ECRECOVER
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
    ))

;;  PRC/may_only_fail_in_RAM
(defun (scenario-shorthand---PRC---may-only-fail-in-the-RAM)
  (+
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
    ))

;;  PRC/weighted_sum
(defun (scenario-shorthand---PRC---weighted-address-bit-sum)
  (+
    (*  1  scenario/PRC_ECRECOVER  )
    (*  2  scenario/PRC_SHA2-256   )
    (*  3  scenario/PRC_RIPEMD-160 )
    (*  4  scenario/PRC_IDENTITY   )
    (*  5  scenario/PRC_MODEXP     )
    (*  6  scenario/PRC_ECADD      )
    (*  7  scenario/PRC_ECMUL      )
    (*  8  scenario/PRC_ECPAIRING  )
    (*  9  scenario/PRC_BLAKE2f    )
    ;; scenario/PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/PRC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/PRC_FAILURE_KNOWN_TO_HUB
    ;; scenario/PRC_FAILURE_KNOWN_TO_RAM
    ))

;;  PRC/failure
(defun (scenario-shorthand---PRC---failure)
  (+
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
    ))

;;  PRC/success
(defun (scenario-shorthand---PRC---success)
  (+
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
    ))

;;  PRC/scenario_sum
(defun (scenario-shorthand---PRC---sum)
  (+
    (scenario-shorthand---PRC---failure)
    (scenario-shorthand---PRC---success)
    ))

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
