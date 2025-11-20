(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X Miscellaneous-rows                  ;;
;;   X.Y Automatic vanishing constraints   ;;
;;   X.Y.Z EXP sub-perspective             ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    misc-rows---automatic-vanishing-of-columns-in-inactive-sub-perspectives---EXP-sub-perspective
                  (:guard   PEEK_AT_MISCELLANEOUS)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    misc/EXP_FLAG
                              (begin
                                (vanishes!                  misc/EXP_INST      )
                                (for k [5]   (vanishes!   [ misc/EXP_DATA k ] )) ;; ""
                                )))
