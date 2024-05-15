(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   10.2 SCEN/CREATE instruction shorthands   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;  CREATE/failure_condition
(defun (scenario-shorthand-CREATE-failure-condition)
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

;;  CREATE/execution_will_revert
(defun (scenario-shorthand-CREATE-execution-will-revert)
  (+ 
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/execution_wont_revert
(defun (scenario-shorthand-CREATE-execution-wont-revert)
  (+ 
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/empty_init_code
(defun (scenario-shorthand-CREATE-empty-init-code)
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

;;  CREATE/nonempty_init_code
(defun (scenario-shorthand-CREATE-nonempty-init-code)
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

;;  CREATE/execution
(defun (scenario-shorthand-CREATE-execution)
  (+ 
    (scenario-shorthand-CREATE-nonempty-init-code)
    (scenario-shorthand-CREATE-empty-init-code)
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/undo_account_operations
(defun (scenario-shorthand-CREATE-undo-account-operations)
  (+ 
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/compute_deployment_address
(defun (scenario-shorthand-CREATE-compute-deployment-address)
  (+ 
    (scenario-shorthand-CREATE-failure-condition)
    (scenario-shorthand-CREATE-execution)
    ;; scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    ))

;;  CREATE/unexceptional
(defun (scenario-shorthand-CREATE-unexceptional)
  (+ 
    ;; scenario/CREATE_EXCEPTION
    scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    (scenario-shorthand-CREATE-failure-condition)
    (scenario-shorthand-CREATE-execution)
    ))

;;  CREATE/sum
(defun (scenario-shorthand-CREATE-sum)
  (+ 
    scenario/CREATE_EXCEPTION
    ;; scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    (scenario-shorthand-CREATE-unexceptional)
    ))

;;  CREATE/no_context_change
(defun (scenario-shorthand-CREATE-no-context-change)
  (+ 
    ;; scenario/CREATE_EXCEPTION
    scenario/CREATE_ABORT
    ;; scenario/CREATE_FAILURE_CONDITION_WILL_REVERT
    ;; scenario/CREATE_FAILURE_CONDITION_WONT_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT
    ;; scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT
    ;; scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT
    (scenario-shorthand-CREATE-failure-condition)
    (scenario-shorthand-CREATE-empty-init-code)
    ))

;;  CREATE/deployment_success
(defun (scenario-shorthand-CREATE-deployment-success)
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
(defun (scenario-shorthand-CREATE-deployment-failure)
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


;; ;;  CREATE/
;; (defun (scenario-shorthand-CREATE-)
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
