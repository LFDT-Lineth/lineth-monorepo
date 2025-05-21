(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   10.5 SCEN/PRC instruction shorthands   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;  SELFDESTRUCT/wont_revert
(defun (scenario-shorthand---SELFDESTRUCT---wont-revert) (+ scenario/SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED
                                                            scenario/SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED))

;;  SELFDESTRUCT/unexceptional
(defun (scenario-shorthand---SELFDESTRUCT---unexceptional) (+ scenario/SELFDESTRUCT_WILL_REVERT
                                                              (scenario-shorthand---SELFDESTRUCT---wont-revert)))

;;  SELFDESTRUCT/sum
(defun (scenario-shorthand---SELFDESTRUCT---sum) (+ scenario/SELFDESTRUCT_EXCEPTION
                                                    (scenario-shorthand---SELFDESTRUCT---unexceptional)))
