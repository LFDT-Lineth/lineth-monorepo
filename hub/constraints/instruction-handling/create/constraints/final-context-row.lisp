(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    X.Y.16 Final context row   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    create-instruction---final-context-row-for-exceptional-CREATEs                             (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_EXCEPTION                              ))
                  (execution-provides-empty-return-data       CREATE_exception_caller_context_row___row_offset))

(defconstraint    create-instruction---final-context-row-for-aborted-CREATEs                                 (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_ABORT                                  ))
                  (nonexecution-provides-empty-return-data    CREATE_abort_current_context_row___row_offset))

(defconstraint    create-instruction---final-context-row-when-raising-failure-condition-and-reverting        (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_FAILURE_CONDITION_WILL_REVERT          ))
                  (nonexecution-provides-empty-return-data    CREATE_fcond_will_revert_current_context_row___row_offset))

(defconstraint    create-instruction---final-context-row-when-raising-failure-condition-but-not-reverting    (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_FAILURE_CONDITION_WONT_REVERT          ))
                  (nonexecution-provides-empty-return-data    CREATE_fcond_wont_revert_current_context_row___row_offset))

(defconstraint    create-instruction---final-context-row-with-empty-init-code-and-reverting                  (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT            ))
                  (nonexecution-provides-empty-return-data    CREATE_empty_init_code_will_revert_current_context_row___row_offset))

(defconstraint    create-instruction---final-context-row-with-empty-init-code-but-not-reverting              (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT            ))
                  (nonexecution-provides-empty-return-data    CREATE_empty_init_code_wont_revert_current_context_row___row_offset))

(defconstraint    create-instruction---final-context-row-for-deployment-failures-that-will-revert            (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT ))
                  (begin
                    (initialize-deployment-context      CREATE_nonempty_init_code_failure_will_revert_new_context_row___row_offset)
                    (first-row-of-deployment-context    CREATE_nonempty_init_code_failure_will_revert_new_context_row___row_offset)
                    (justify-createe-revert-data        CREATE_nonempty_init_code_failure_will_revert_new_context_row___row_offset)
                    ))

(defconstraint    create-instruction---final-context-row-for-deployment-failures-that-wont-revert            (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT ))
                  (begin
                    (initialize-deployment-context      CREATE_nonempty_init_code_failure_wont_revert_new_context_row___row_offset)
                    (first-row-of-deployment-context    CREATE_nonempty_init_code_failure_wont_revert_new_context_row___row_offset)
                    (justify-createe-revert-data        CREATE_nonempty_init_code_failure_wont_revert_new_context_row___row_offset)
                    ))

(defconstraint    create-instruction---final-context-row-for-deployment-successes-that-will-revert           (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT ))
                  (begin
                    (initialize-deployment-context      CREATE_nonempty_init_code_success_will_revert_new_context_row___row_offset)
                    (first-row-of-deployment-context    CREATE_nonempty_init_code_success_will_revert_new_context_row___row_offset)
                    (justify-createe-revert-data        CREATE_nonempty_init_code_success_will_revert_new_context_row___row_offset)
                    ))

(defconstraint    create-instruction---final-context-row-for-deployment-successes-that-wont-revert           (:guard    (*    PEEK_AT_SCENARIO    scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT ))
                  (begin
                    (initialize-deployment-context      CREATE_nonempty_init_code_success_wont_revert_new_context_row___row_offset)
                    (first-row-of-deployment-context    CREATE_nonempty_init_code_success_wont_revert_new_context_row___row_offset)
                    (justify-createe-revert-data        CREATE_nonempty_init_code_success_wont_revert_new_context_row___row_offset)
                    ))

(defun    (initialize-deployment-context    relative_row_offset)
  (initialize-context
         relative_row_offset                                                                           ;; row offset
         (+    1    HUB_STAMP)                                                                         ;; context number
         (+    1    (create-instruction---current-context-csd))                                        ;; call stack depth
         0                                                                                             ;; is root
         0                                                                                             ;; is static
         (create-instruction---createe-address-hi)                                                     ;; account address high
         (create-instruction---createe-address-lo)                                                     ;; account address low
         (shift    account/DEPLOYMENT_NUMBER_NEW    CREATE_first_createe_account_row___row_offset)     ;; account deployment number
         (create-instruction---createe-address-hi)                                                     ;; byte code address high
         (create-instruction---createe-address-lo)                                                     ;; byte code address low
         (shift    account/DEPLOYMENT_NUMBER_NEW    CREATE_first_createe_account_row___row_offset)     ;; byte code deployment number
         1                                                                                             ;; byte code deployment status
         (create-instruction---deployment-cfi)                                                         ;; byte code code fragment index
         (create-instruction---creator-address-hi)                                                     ;; caller address high
         (create-instruction---creator-address-lo)                                                     ;; caller address low
         (create-instruction---STACK-value-lo)                                                         ;; call value
         CONTEXT_NUMBER                                                                                ;; caller context
         0                                                                                             ;; call data offset
         0                                                                                             ;; call data size
         0                                                                                             ;; return at offset
         0                                                                                             ;; return at capacity
         ))


(defun    (first-row-of-deployment-context    relative_row_offset)
  (begin
    (eq!    (shift    CONTEXT_NUMBER           (+    relative_row_offset    1))    (shift    context/CONTEXT_NUMBER                   relative_row_offset))
    (eq!    (shift    CALLER_CONTEXT_NUMBER    (+    relative_row_offset    1))    (shift    CONTEXT_NUMBER                           relative_row_offset))
    (eq!    (shift    CFI                      (+    relative_row_offset    1))    (shift    context/BYTE_CODE_CODE_FRAGMENT_INDEX    relative_row_offset))
    (eq!    (shift    PROGRAM_COUNTER          (+    relative_row_offset    1))    0)
    (eq!    (shift    GAS_XPCT                 (+    relative_row_offset    1))    (create-instruction---STP-gas-paid-out-of-pocket))
    ))

(defun    (justify-createe-revert-data    relative_row_offset)
  (begin
    (eq!    (create-instruction---createe-self-reverts)    (shift    CONTEXT_SELF_REVERTS    (+    relative_row_offset    1)))
    (eq!    (create-instruction---createe-revert-stamp)    (shift    CONTEXT_REVERT_STAMP    (+    relative_row_offset    1)))
    ))
