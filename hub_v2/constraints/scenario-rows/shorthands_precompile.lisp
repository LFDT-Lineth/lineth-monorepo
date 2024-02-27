(module hub_v2)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   10.5 SCEN/PRC instruction shorthands   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;  PRC/sum
(defun (scen-PRC-shorthand-sum)
  (+ 
    PRC_ECRECOVER
    PRC_SHA2-256
    PRC_RIPEMD-160
    PRC_IDENTITY
    PRC_MODEXP
    PRC_ECADD
    PRC_ECMUL
    PRC_ECPAIRING
    PRC_BLAKE2f
    ;; PRC_SUCCESS_WILL_REVERT
    ;; PRC_SUCCESS_WONT_REVERT
    ;; PRC_FAILURE_KNOWN_TO_HUB
    ;; PRC_FAILURE_KNOWN_TO_RAM
    ))

;;  PRC/weighted_sum
(defun (scen-PRC-shorthand-weighted-sum)
  (+ 
    (*  1  PRC_ECRECOVER  )
    (*  2  PRC_SHA2-256   )
    (*  3  PRC_RIPEMD-160 )
    (*  4  PRC_IDENTITY   )
    (*  5  PRC_MODEXP     )
    (*  6  PRC_ECADD      )
    (*  7  PRC_ECMUL      )
    (*  8  PRC_ECPAIRING  )
    (*  9  PRC_BLAKE2f    )
    ;; PRC_SUCCESS_WILL_REVERT
    ;; PRC_SUCCESS_WONT_REVERT
    ;; PRC_FAILURE_KNOWN_TO_HUB
    ;; PRC_FAILURE_KNOWN_TO_RAM
    ))

;;  PRC/failure
(defun (scen-PRC-shorthand-failure)
  (+ 
    ;; PRC_ECRECOVER
    ;; PRC_SHA2-256
    ;; PRC_RIPEMD-160
    ;; PRC_IDENTITY
    ;; PRC_MODEXP
    ;; PRC_ECADD
    ;; PRC_ECMUL
    ;; PRC_ECPAIRING
    ;; PRC_BLAKE2f
    ;; PRC_SUCCESS_WILL_REVERT
    ;; PRC_SUCCESS_WONT_REVERT
    PRC_FAILURE_KNOWN_TO_HUB
    PRC_FAILURE_KNOWN_TO_RAM
    ))

;;  PRC/success
(defun (scen-PRC-shorthand-success)
  (+ 
    ;; PRC_ECRECOVER
    ;; PRC_SHA2-256
    ;; PRC_RIPEMD-160
    ;; PRC_IDENTITY
    ;; PRC_MODEXP
    ;; PRC_ECADD
    ;; PRC_ECMUL
    ;; PRC_ECPAIRING
    ;; PRC_BLAKE2f
    PRC_SUCCESS_WILL_REVERT
    PRC_SUCCESS_WONT_REVERT
    ;; PRC_FAILURE_KNOWN_TO_HUB
    ;; PRC_FAILURE_KNOWN_TO_RAM
    ))

;;  PRC/scenario_sum
(defun (scen-PRC-shorthand-scenario-sum)
  (+ 
    ;; PRC_ECRECOVER
    ;; PRC_SHA2-256
    ;; PRC_RIPEMD-160
    ;; PRC_IDENTITY
    ;; PRC_MODEXP
    ;; PRC_ECADD
    ;; PRC_ECMUL
    ;; PRC_ECPAIRING
    ;; PRC_BLAKE2f
    (scen-PRC-shorthand-failure)
    (scen-PRC-shorthand-success)
    ))

;; ;;  PRC/
;; (defun (scen-PRC-shorthand-)
;;   (+ 
;;     PRC_ECRECOVER
;;     PRC_SHA2-256
;;     PRC_RIPEMD-160
;;     PRC_IDENTITY
;;     PRC_MODEXP
;;     PRC_ECADD
;;     PRC_ECMUL
;;     PRC_ECPAIRING
;;     PRC_BLAKE2f
;;     PRC_SUCCESS_WILL_REVERT
;;     PRC_SUCCESS_WONT_REVERT
;;     PRC_FAILURE_KNOWN_TO_HUB
;;     PRC_FAILURE_KNOWN_TO_RAM
;;     ))
