(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;   X Miscellaneous-rows                  ;;
;;   X.Y Automatic vanishing constraints   ;;
;;   X.Y.Z OOB sub-perspective             ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    misc-rows---automatic-vanishing-of-columns-in-inactive-sub-perspectives---OOB-sub-perspective
                  (:guard   PEEK_AT_MISCELLANEOUS)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-zero    misc/OOB_FLAG
                              (begin
                                (vanishes!                    misc/OOB_INST        )
                                (for k [1:10]  (vanishes!   [ misc/OOB_DATA   k ] )) ;; ""
                                )))
