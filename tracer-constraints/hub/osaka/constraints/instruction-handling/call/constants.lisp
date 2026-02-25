(module hub)

(defconst

  ;; CALL specific row offset constants
  CALL_1st_stack_row___row_offset                                              -2
  CALL_2nd_stack_row___row_offset                                              -1
  CALL_1st_scenario_row___row_offset                                            0
  CALL_1st_context_row___row_offset                                             1
  CALL_misc_row___row_offset                                                    2
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  CALL_1st_caller_account_row___row_offset                                      3
  CALL_1st_callee_account_row___row_offset                                      4
  CALL_1st_delegt_account_row___row_offset                                      5
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  CALL_2nd_callee_account_row___abort_will_revert___row_offset                  6
  CALL_2nd_delegt_account_row___abort_will_revert___row_offset                  7
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  CALL_2nd_caller_account_row___row_offset                                      6
  CALL_2nd_callee_account_row___row_offset                                      7
  CALL_2nd_delegt_account_row___row_offset                                      8
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  CALL_3rd_callee_account_row___row_offset                                      9
  CALL_3rd_delegt_account_row___row_offset                                     10
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  CALL_staticx_update_parent_context_row___row_offset                           3
  CALL_mxpx_update_parent_context_row___row_offset                              3
  CALL_oogx_update_parent_context_row___row_offset                              6
  CALL_ABORT_WILL_REVERT---current-context-update---row-offset                  8
  CALL_ABORT_WONT_REVERT---current-context-update---row-offset                  6
  CALL_EOA_will_revert_caller_context_row___row_offset                          9
  CALL_EOA_wont_revert_caller_context_row___row_offset                          6
  CALL_SMC_failure_will_revert_initialize_callee_context_row___row_offset      11
  CALL_SMC_failure_wont_revert_initialize_callee_context_row___row_offset       9
  CALL_SMC_success_will_revert_initialize_callee_context_row___row_offset       9
  CALL_SMC_success_wont_revert_initialize_callee_context_row___row_offset       6
  ;;
  CALL_2nd_scenario_row_PRC_failure___row_offset                                6
  CALL_2nd_scenario_row_PRC_success_will_revert_2nd_scenario___row_offset       9
  CALL_2nd_scenario_row_PRC_success_wont_revert_2nd_scenario___row_offset       6

  ;; NSR's for non-entry or non precompiles
  CALL_nsr___staticx                                                            (+  CALL_staticx_update_parent_context_row___row_offset                      1 )
  CALL_nsr___mxpx                                                               (+  CALL_mxpx_update_parent_context_row___row_offset                         1 )
  CALL_nsr___oogx                                                               (+  CALL_oogx_update_parent_context_row___row_offset                         1 )
  CALL_nsr___abort_will_revert                                                  (+  CALL_ABORT_WILL_REVERT---current-context-update---row-offset             1 )
  CALL_nsr___abort_wont_revert                                                  (+  CALL_ABORT_WONT_REVERT---current-context-update---row-offset             1 )
  CALL_nsr___eoa_success_will_revert                                            (+  CALL_EOA_will_revert_caller_context_row___row_offset                     1 )
  CALL_nsr___eoa_success_wont_revert                                            (+  CALL_EOA_wont_revert_caller_context_row___row_offset                     1 )
  CALL_nsr___smc_failure_will_revert                                            (+  CALL_SMC_failure_will_revert_initialize_callee_context_row___row_offset  1 )
  CALL_nsr___smc_failure_wont_revert                                            (+  CALL_SMC_failure_wont_revert_initialize_callee_context_row___row_offset  1 )
  CALL_nsr___smc_success_will_revert                                            (+  CALL_SMC_success_will_revert_initialize_callee_context_row___row_offset  1 )
  CALL_nsr___smc_success_wont_revert                                            (+  CALL_SMC_success_wont_revert_initialize_callee_context_row___row_offset  1 )

  ;; partial NSR's for entry into precompiles
  CALL___first_half_nsr___prc_failure                                           CALL_2nd_scenario_row_PRC_failure___row_offset
  CALL___first_half_nsr___prc_success_will_revert                               CALL_2nd_scenario_row_PRC_success_will_revert_2nd_scenario___row_offset
  CALL___first_half_nsr___prc_success_wont_revert                               CALL_2nd_scenario_row_PRC_success_wont_revert_2nd_scenario___row_offset
  )
