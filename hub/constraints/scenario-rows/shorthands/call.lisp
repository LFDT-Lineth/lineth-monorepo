(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;   10.2 SCEN/CALL instruction shorthands   ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;  CALL/externally_owned_account
(defun (scenario-shorthand---CALL---externally-owned-account)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/smart_contract
(defun (scenario-shorthand---CALL---smart-contract)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/precompile
(defun (scenario-shorthand---CALL---precompile)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_PRC_FAILURE
    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/abort
(defun (scenario-shorthand---CALL---abort)
  (+
    scenario/CALL_ABORT_WILL_REVERT
    scenario/CALL_ABORT_WONT_REVERT
    ))

;;  CALL/entry
(defun (scenario-shorthand---CALL---entry)
  (+
    (scenario-shorthand---CALL---externally-owned-account)
    (scenario-shorthand---CALL---smart-contract)
    (scenario-shorthand---CALL---precompile)
    ))

;;  CALL/unexceptional
(defun (scenario-shorthand---CALL---unexceptional)
  (+
    (scenario-shorthand---CALL---abort)
    (scenario-shorthand---CALL---entry)
    ))

;;  CALL/sum
(defun (scenario-shorthand---CALL---sum)
  (+
    scenario/CALL_EXCEPTION
    (scenario-shorthand---CALL---unexceptional)
    ))

;;  CALL/no_precompile
(defun (scenario-shorthand---CALL---no-precompile)
  (+
    scenario/CALL_EXCEPTION
    (scenario-shorthand---CALL---abort)
    (scenario-shorthand---CALL---externally-owned-account)
    (scenario-shorthand---CALL---smart-contract)
    ))

;;  CALL/precompile_success
(defun (scenario-shorthand---CALL---precompile-success)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/execution_known_to_revert
(defun (scenario-shorthand---CALL---execution-known-to-revert)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/execution_known_to_not_revert
(defun (scenario-shorthand---CALL---execution-known-to-not-revert)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/success
(defun (scenario-shorthand---CALL---success)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/smc_success
(defun (scenario-shorthand---CALL---smc-success)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/smc_failure
(defun (scenario-shorthand---CALL---smc-failure)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/failure
(defun (scenario-shorthand---CALL---failure)
  (+
    (scenario-shorthand---CALL---smc-failure)
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/no_context_change
(defun (scenario-shorthand---CALL---no-context-change)
  (+
    ;; scenario/CALL_EXCEPTION
    (scenario-shorthand---CALL---abort)
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    (scenario-shorthand---CALL---externally-owned-account)
    (scenario-shorthand---CALL---precompile)
    ))

;;  CALL/callee_warmth_update_not_required
(defun (scenario-shorthand---CALL---callee-warmth-update-not-required)
  (+    scenario/CALL_EXCEPTION    0))    ;; TODO: test if removing the (+ ... 0) causes compilation issues; it certainly breaks the syntax highlighting :/

;;  CALL/callee_warmth_update_required
(defun (scenario-shorthand---CALL---callee-warmth-update-required)
  (+    (scenario-shorthand---CALL---abort)
        (scenario-shorthand---CALL---entry)))

;;  CALL/balance_update_not_required
(defun (scenario-shorthand---CALL---balance-update-not-required)
  (+
    scenario/CALL_EXCEPTION
    (scenario-shorthand---CALL---abort)
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))


;;  CALL/balance_update_required
(defun (scenario-shorthand---CALL---balance-update-required)
  (+
    (scenario-shorthand---CALL---externally-owned-account)
    (scenario-shorthand---CALL---smart-contract)
    (scenario-shorthand---CALL---precompile-success)
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/requires_both_accounts_twice
(defun (scenario-shorthand---CALL---requires-both-accounts-twice)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/undoes_balance_update_with_failure
(defun (scenario-shorthand---CALL---balance-update-undone-with-callee-failure)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;;  CALL/undoes_balance_update_with_revert
(defun (scenario-shorthand---CALL---balance-update-undone-with-caller-revert)
  (+
    ;; scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
    scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
    scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
    ;; scenario/CALL_PRC_FAILURE
    scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
    ;; scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
    ))

;; ;;  CALL/
;; (defun (scenario-shorthand---CALL---)
;;   (+
;;     scenario/CALL_EXCEPTION
    ;; scenario/CALL_ABORT_WILL_REVERT
    ;; scenario/CALL_ABORT_WONT_REVERT
;;     scenario/CALL_EOA_SUCCESS_CALLER_WILL_REVERT
;;     scenario/CALL_EOA_SUCCESS_CALLER_WONT_REVERT
;;     scenario/CALL_SMC_FAILURE_CALLER_WILL_REVERT
;;     scenario/CALL_SMC_FAILURE_CALLER_WONT_REVERT
;;     scenario/CALL_SMC_SUCCESS_CALLER_WILL_REVERT
;;     scenario/CALL_SMC_SUCCESS_CALLER_WONT_REVERT
;;     scenario/CALL_PRC_FAILURE
;;     scenario/CALL_PRC_SUCCESS_CALLER_WILL_REVERT
;;     scenario/CALL_PRC_SUCCESS_CALLER_WONT_REVERT
;;     ))
