(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X Miscellaneous-rows                  ;;
;;   X.Y Automatic vanishing constraints   ;;
;;   X.Y.Z MMU sub-perspective             ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    misc-rows---automatic-vanishing-of-columns-in-inactive-sub-perspectives---MMU-sub-perspective
                  (:guard   PEEK_AT_MISCELLANEOUS)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    misc/MMU_FLAG
                              (begin
                                (vanishes!   misc/MMU_INST          )
                                (vanishes!   misc/MMU_SRC_ID        )
                                (vanishes!   misc/MMU_TGT_ID        )
                                (vanishes!   misc/MMU_AUX_ID        )
                                (vanishes!   misc/MMU_SRC_OFFSET_LO )
                                (vanishes!   misc/MMU_SRC_OFFSET_HI )
                                (vanishes!   misc/MMU_TGT_OFFSET_LO )
                                (vanishes!   misc/MMU_SIZE          )
                                (vanishes!   misc/MMU_REF_OFFSET    )
                                (vanishes!   misc/MMU_REF_SIZE      )
                                (vanishes!   misc/MMU_SUCCESS_BIT   )
                                (vanishes!   misc/MMU_LIMB_1        )
                                (vanishes!   misc/MMU_LIMB_2        )
                                (vanishes!   misc/MMU_PHASE         )
                                (vanishes!   misc/MMU_EXO_SUM       )
                                )))
