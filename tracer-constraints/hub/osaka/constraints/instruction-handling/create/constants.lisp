(module hub)

(defconst
  CREATE_first_stack_row___row_offset                                           -2
  CREATE_second_stack_row___row_offset                                          -1
  CREATE_current_context_row___row_offset                                        1
  CREATE_miscellaneous_row___row_offset                                          2
  CREATE_first_creator_account_row___row_offset                                  3
  CREATE_first_createe_account_row___row_offset                                  4
  CREATE_second_creator_account_row___row_offset                                 5
  CREATE_second_createe_account_row___row_offset                                 6
  CREATE_third_creator_account_row___row_offset                                  7
  CREATE_third_createe_account_row___row_offset                                  8
  ;; Exception
  CREATE_exception_caller_context_row___row_offset                               3
  ;; Abort
  CREATE_abort_current_account_row___row_offset                                  3
  CREATE_abort_current_context_row___row_offset                                  4
  ;; Failure condition (will or won't revert)
  CREATE_fcond_will_revert_current_context_row___row_offset                      7
  CREATE_fcond_wont_revert_current_context_row___row_offset                      5
  ;; Empty initialization code (will or won't revert)
  CREATE_empty_init_code_will_revert_current_context_row___row_offset            7
  CREATE_empty_init_code_wont_revert_current_context_row___row_offset            5
  ;; Nonempty initialization code failure (will or won't revert)
  CREATE_nonempty_init_code_failure_will_revert_new_context_row___row_offset     9
  CREATE_nonempty_init_code_failure_wont_revert_new_context_row___row_offset     7
  ;; Nonempty initialization code success (will or won't revert)
  CREATE_nonempty_init_code_success_will_revert_new_context_row___row_offset     7
  CREATE_nonempty_init_code_success_wont_revert_new_context_row___row_offset     5
  )

