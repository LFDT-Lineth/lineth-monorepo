(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   10.2 SCEN/CREATE instruction shorthands   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;






;;  CREATE/not_rebuffed_empty_init_code
(defun (scenario-shorthand---CREATE---not-rebuffed-empty-init-code)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/not_rebuffed_nonempty_init_code
(defun (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/not_rebuffed
(defun (scenario-shorthand---CREATE---not-rebuffed)
  (+
    (scenario-shorthand---CREATE---not-rebuffed-empty-init-code)
    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
    ))

;;  CREATE/rebuffed
(defun (scenario-shorthand---CREATE---rebuffed)
  (+
    scenario/CREATE_EXCEPTION
    scenario/CREATE_ABORT
    (scenario-shorthand---CREATE---failure-condition)
    ))







;;  CREATE/failure_condition
(defun (scenario-shorthand---CREATE---failure-condition)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/unexceptional
(defun (scenario-shorthand---CREATE---unexceptional)
  (+
    scenario/CREATE_ABORT
    (scenario-shorthand---CREATE---failure-condition)
    (scenario-shorthand---CREATE---not-rebuffed)
    ))

;;  CREATE/sum
(defun (scenario-shorthand---CREATE---sum)
  (+
    scenario/CREATE_EXCEPTION
    (scenario-shorthand---CREATE---unexceptional)
    ))












;;  CREATE/compute_deployment_address
(defun (scenario-shorthand---CREATE---compute-deployment-address)
  (+
    (scenario-shorthand---CREATE---failure-condition)
    (scenario-shorthand---CREATE---not-rebuffed)
    ))

;;  CREATE/load_createe_account
(defun (scenario-shorthand---CREATE---load-createe-account)
  (+
    (scenario-shorthand---CREATE---failure-condition)
    (scenario-shorthand---CREATE---not-rebuffed)
    ))

;;  CREATE/no_context_change
(defun (scenario-shorthand---CREATE---no-context-change)
  (+
    scenario/CREATE_ABORT
    (scenario-shorthand---CREATE---failure-condition)
    (scenario-shorthand---CREATE---not-rebuffed-empty-init-code)
    ))













;;  CREATE/deployment-success
(defun (scenario-shorthand---CREATE---deployment-success)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/deployment_failure
(defun (scenario-shorthand---CREATE---deployment-failure)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))












;;  CREATE/creator_state_change_will_revert
(defun (scenario-shorthand---CREATE---creator-state-change-will-revert)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/creator_state_change_wont_revert
(defun (scenario-shorthand---CREATE---creator-state-change-wont-revert)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/creator_state_change
(defun (scenario-shorthand---CREATE---creator-state-change)
  (+
    (scenario-shorthand---CREATE---failure-condition)
    (scenario-shorthand---CREATE---not-rebuffed)
    ))

;;  CREATE/no_creator_state_change
(defun (scenario-shorthand---CREATE---no-creator-state-change)
  (+
    scenario/CREATE_EXCEPTION
    scenario/CREATE_ABORT
    ))












;;  CREATE/simple_revert
(defun (scenario-shorthand---CREATE---simple-revert)
  (+
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))


;; ;;  CREATE/execution_will_revert
;; (defun (scenario-shorthand---CREATE---execution-will-revert)
;;   (+
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
;;
;; ;;  CREATE/execution_wont_revert
;; (defun (scenario-shorthand---CREATE---execution-wont-revert)
;;   (+
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
;;
;; ;;  CREATE/empty_init_code
;; (defun (scenario-shorthand---CREATE---empty-init-code)
;;   (+
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
;;
;; ;;  CREATE/nonempty_init_code
;; (defun (scenario-shorthand---CREATE---nonempty-init-code)
;;   (+
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
;;
;; ;;  CREATE/execution
;; (defun (scenario-shorthand---CREATE---execution)
;;   (+
;;     (scenario-shorthand---CREATE---nonempty-init-code)
;;     (scenario-shorthand---CREATE---empty-init-code)
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
;;
;; ;;  CREATE/undo_account_operations
;; (defun (scenario-shorthand---CREATE---undo-account-operations)
;;   (+
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
;;
;; ;;  CREATE/deployment_success
;; (defun (scenario-shorthand---CREATE---deployment-success)
;;   (+
;;     ;; scenario/CREATE_EXCEPTION
;;     ;; scenario/CREATE_ABORT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))


;; ;;  CREATE/
;; (defun (scenario-shorthand---CREATE---)
;;   (+
;;     scenario/CREATE_EXCEPTION
;;     scenario/CREATE_ABORT
;;     scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
;;     scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
;;     scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
;;     scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
;;     ))
