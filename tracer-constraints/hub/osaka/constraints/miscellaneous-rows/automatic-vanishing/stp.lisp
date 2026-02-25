(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X Miscellaneous-rows                  ;;
;;   X.Y Automatic vanishing constraints   ;;
;;   X.Y.Z STP sub-perspective             ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    misc-rows---automatic-vanishing-of-columns-in-inactive-sub-perspectives---STP-sub-perspective
                  (:guard   PEEK_AT_MISCELLANEOUS)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    misc/STP_FLAG
                              (begin
                                (vanishes!   misc/STP_INSTRUCTION                  )
                                (vanishes!   misc/STP_GAS_HI                       )
                                (vanishes!   misc/STP_GAS_LO                       )
                                (vanishes!   misc/STP_VALUE_HI                     )
                                (vanishes!   misc/STP_VALUE_LO                     )
                                (vanishes!   misc/STP_EXISTS                       )
                                (vanishes!   misc/STP_WARMTH                       )
                                (vanishes!   misc/STP_CALLEE_IS_DELEGATED          )
                                (vanishes!   misc/STP_CALLEE_IS_DELEGATED_TO_SELF  )
                                (vanishes!   misc/STP_DELEGATE_WARMTH              )
                                (vanishes!   misc/STP_OOGX                         )
                                (vanishes!   misc/STP_GAS_MXP                      )
                                (vanishes!   misc/STP_GAS_UPFRONT_GAS_COST         )
                                (vanishes!   misc/STP_GAS_PAID_OUT_OF_POCKET       )
                                (vanishes!   misc/STP_GAS_STIPEND                  )
                                )))
