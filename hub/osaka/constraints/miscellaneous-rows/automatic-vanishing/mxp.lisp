(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X Miscellaneous-rows                  ;;
;;   X.Y Automatic vanishing constraints   ;;
;;   X.Y.Z MXP sub-perspective             ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    misc-rows---automatic-vanishing-of-columns-in-inactive-sub-perspectives---MXP-sub-perspective
                  (:guard   PEEK_AT_MISCELLANEOUS)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    misc/MXP_FLAG
                              (begin
                                (vanishes!   misc/MXP_INST                   )
                                (vanishes!   misc/MXP_MXPX                   )
                                (vanishes!   misc/MXP_DEPLOYS                )
                                (vanishes!   misc/MXP_OFFSET_1_HI            )
                                (vanishes!   misc/MXP_OFFSET_1_LO            )
                                (vanishes!   misc/MXP_OFFSET_2_HI            )
                                (vanishes!   misc/MXP_OFFSET_2_LO            )
                                (vanishes!   misc/MXP_SIZE_1_HI              )
                                (vanishes!   misc/MXP_SIZE_1_LO              )
                                (vanishes!   misc/MXP_SIZE_2_HI              )
                                (vanishes!   misc/MXP_SIZE_2_LO              )
                                (vanishes!   misc/MXP_WORDS                  )
                                (vanishes!   misc/MXP_GAS_MXP                )
                                (vanishes!   misc/MXP_SIZE_1_NONZERO_NO_MXPX )
                                (vanishes!   misc/MXP_SIZE_2_NONZERO_NO_MXPX )
                                )))

