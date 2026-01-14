(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;    X.Y.6 Non stack rows and peeking flags   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    create-instruction---setting-NSR   (:guard    (create-instruction---standard-precondition))
                  (eq!    (shift    NON_STACK_ROWS    CREATE_first_stack_row___row_offset)
                          (+    (*  (+  1  CREATE_exception_caller_context_row___row_offset                           )  scenario/CREATE_EXCEPTION                              )
                                (*  (+  1  CREATE_abort_current_context_row___row_offset                              )  scenario/CREATE_ABORT                                  )
                                (*  (+  1  CREATE_fcond_will_revert_current_context_row___row_offset                  )  scenario/CREATE_FAILURE_CONDITION_WILL_REVERT          )
                                (*  (+  1  CREATE_fcond_wont_revert_current_context_row___row_offset                  )  scenario/CREATE_FAILURE_CONDITION_WONT_REVERT          )
                                (*  (+  1  CREATE_empty_init_code_will_revert_current_context_row___row_offset        )  scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT            )
                                (*  (+  1  CREATE_empty_init_code_wont_revert_current_context_row___row_offset        )  scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT            )
                                (*  (+  1  CREATE_nonempty_init_code_failure_will_revert_new_context_row___row_offset )  scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT )
                                (*  (+  1  CREATE_nonempty_init_code_failure_wont_revert_new_context_row___row_offset )  scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT )
                                (*  (+  1  CREATE_nonempty_init_code_success_will_revert_new_context_row___row_offset )  scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT )
                                (*  (+  1  CREATE_nonempty_init_code_success_wont_revert_new_context_row___row_offset )  scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT )
                                )
                          )
                  )

(defconstraint    create-instruction---setting-the-peeking-flags    (:guard    (create-instruction---standard-precondition))
                  (eq!    (shift    NON_STACK_ROWS    CREATE_first_stack_row___row_offset)
                          (+    (*  (create-instruction---flag-sum-staticx)                            (create-instruction---STACK-staticx)                   )
                          (+    (*  (create-instruction---flag-sum-maxcsx)                             (create-instruction---STACK-maxcsx)                    )
                                (*  (create-instruction---flag-sum-mxpx)                               (create-instruction---STACK-mxpx)                      )
                                (*  (create-instruction---flag-sum-oogx)                               (create-instruction---STACK-oogx)                      )
                                (*  (create-instruction---flag-sum-abort)                              scenario/CREATE_ABORT                                  )
                                (*  (create-instruction---flag-sum-fcond-will-revert)                  scenario/CREATE_FAILURE_CONDITION_WILL_REVERT          )
                                (*  (create-instruction---flag-sum-fcond-wont-revert)                  scenario/CREATE_FAILURE_CONDITION_WONT_REVERT          )
                                (*  (create-instruction---flag-sum-empty-init-code-will-revert)        scenario/CREATE_EMPTY_INIT_CODE_WILL_REVERT            )
                                (*  (create-instruction---flag-sum-empty-init-code-wont-revert)        scenario/CREATE_EMPTY_INIT_CODE_WONT_REVERT            )
                                (*  (create-instruction---flag-sum-nonempty-init-failure-will-revert)  scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT )
                                (*  (create-instruction---flag-sum-nonempty-init-failure-wont-revert)  scenario/CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT )
                                (*  (create-instruction---flag-sum-nonempty-init-success-will-revert)  scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT )
                                (*  (create-instruction---flag-sum-nonempty-init-success-wont-revert)  scenario/CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT )
                                )
                          )
                  ))
