(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    X.Y.5 Peeking flag shorthands   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---standard-precondition)    (*    PEEK_AT_SCENARIO
                                                                (scenario-shorthand---CREATE---sum)))

(defun    (create-instruction---std-prefix)          (+   PEEK_AT_SCENARIO
                                                          (shift    PEEK_AT_CONTEXT         CREATE_current_context_row___row_offset)
                                                          (shift    PEEK_AT_MISCELLANEOUS   CREATE_miscellaneous_row___row_offset)
                                                          ))

(defun    (create-instruction---flag-sum-staticx)    (+   (create-instruction---std-prefix)
                                                          (shift    PEEK_AT_CONTEXT         CREATE_exception_caller_context_row___row_offset)
                                                          ))

(defun    (create-instruction---flag-sum-maxcsx)     (+   (create-instruction---std-prefix)
                                                          (shift    PEEK_AT_CONTEXT         CREATE_exception_caller_context_row___row_offset)
                                                          ))

(defun    (create-instruction---flag-sum-mxpx)       (+   (create-instruction---std-prefix)
                                                          (shift    PEEK_AT_CONTEXT         CREATE_exception_caller_context_row___row_offset)
                                                          ))

(defun    (create-instruction---flag-sum-oogx)       (+   (create-instruction---std-prefix)
                                                          (shift    PEEK_AT_CONTEXT         CREATE_exception_caller_context_row___row_offset)
                                                          ))

(defun    (create-instruction---flag-sum-abort)      (+   (create-instruction---std-prefix)
                                                          (shift    PEEK_AT_ACCOUNT         CREATE_abort_current_account_row___row_offset)
                                                          (shift    PEEK_AT_CONTEXT         CREATE_abort_current_context_row___row_offset)
                                                          ))

(defun    (create-instruction---flag-sum-fcond-will-revert)      (+   (create-instruction---std-prefix)
                                                                      (shift    PEEK_AT_ACCOUNT    CREATE_first_creator_account_row___row_offset             )
                                                                      (shift    PEEK_AT_ACCOUNT    CREATE_first_createe_account_row___row_offset             )
                                                                      (shift    PEEK_AT_ACCOUNT    CREATE_second_creator_account_row___row_offset            )
                                                                      (shift    PEEK_AT_ACCOUNT    CREATE_second_createe_account_row___row_offset            )
                                                                      (shift    PEEK_AT_CONTEXT    CREATE_fcond_will_revert_current_context_row___row_offset )
                                                                      ))

(defun    (create-instruction---flag-sum-fcond-wont-revert)      (+   (create-instruction---std-prefix)
                                                                      (shift    PEEK_AT_ACCOUNT    CREATE_first_creator_account_row___row_offset  )
                                                                      (shift    PEEK_AT_ACCOUNT    CREATE_first_createe_account_row___row_offset  )
                                                                      (shift    PEEK_AT_CONTEXT    CREATE_fcond_wont_revert_current_context_row___row_offset )
                                                                      ))

(defun    (create-instruction---sanctioned-prefix)     (+   (create-instruction---std-prefix)
                                                            (shift    PEEK_AT_ACCOUNT    CREATE_first_creator_account_row___row_offset  )
                                                            (shift    PEEK_AT_ACCOUNT    CREATE_first_createe_account_row___row_offset  )
                                                            ))

(defun    (create-instruction---flag-sum-empty-init-code-will-revert)    (+   (create-instruction---sanctioned-prefix)
                                                                              (shift    PEEK_AT_ACCOUNT    CREATE_second_creator_account_row___row_offset                     )
                                                                              (shift    PEEK_AT_ACCOUNT    CREATE_second_createe_account_row___row_offset                     )
                                                                              (shift    PEEK_AT_CONTEXT    CREATE_empty_init_code_will_revert_current_context_row___row_offset)
                                                                              ))

(defun    (create-instruction---flag-sum-empty-init-code-wont-revert)    (+   (create-instruction---sanctioned-prefix)
                                                                              (shift    PEEK_AT_CONTEXT    CREATE_empty_init_code_wont_revert_current_context_row___row_offset)
                                                                              ))

(defun    (create-instruction---flag-sum-nonempty-init-failure-will-revert)    (+   (create-instruction---sanctioned-prefix)
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_second_creator_account_row___row_offset                            )
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_second_createe_account_row___row_offset                            )
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_third_creator_account_row___row_offset                             )
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_third_createe_account_row___row_offset                             )
                                                                                    (shift    PEEK_AT_CONTEXT    CREATE_nonempty_init_code_failure_will_revert_new_context_row___row_offset)
                                                                                    ))

(defun    (create-instruction---flag-sum-nonempty-init-failure-wont-revert)    (+   (create-instruction---sanctioned-prefix)
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_second_creator_account_row___row_offset                            )
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_second_createe_account_row___row_offset                            )
                                                                                    (shift    PEEK_AT_CONTEXT    CREATE_nonempty_init_code_failure_wont_revert_new_context_row___row_offset)
                                                                                    ))

(defun    (create-instruction---flag-sum-nonempty-init-success-will-revert)    (+   (create-instruction---sanctioned-prefix)
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_second_creator_account_row___row_offset                            )
                                                                                    (shift    PEEK_AT_ACCOUNT    CREATE_second_createe_account_row___row_offset                            )
                                                                                    (shift    PEEK_AT_CONTEXT    CREATE_nonempty_init_code_success_will_revert_new_context_row___row_offset)
                                                                                    ))

(defun    (create-instruction---flag-sum-nonempty-init-success-wont-revert)    (+   (create-instruction---sanctioned-prefix)
                                                                                    (shift    PEEK_AT_CONTEXT    CREATE_nonempty_init_code_success_wont_revert_new_context_row___row_offset)
                                                                                    ))
