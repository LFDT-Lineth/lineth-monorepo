(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   X.Y Transaction phase flags   ;;
;;   X.Y.Z Shorthands              ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (phase-flag-sum)    (force-bin   (+    TX_SKIP
                                                 TX_WARM
                                                 TX_AUTH
                                                 TX_INIT
                                                 TX_EXEC
                                                 TX_FINL
                                                 )))
(defun    (phase-wght-sum)                 (+    (*  (^  2  0)  TX_SKIP )
                                                 (*  (^  2  1)  TX_WARM )
                                                 (*  (^  2  2)  TX_AUTH )
                                                 (*  (^  2  3)  TX_INIT )
                                                 (*  (^  2  4)  TX_EXEC )
                                                 (*  (^  2  5)  TX_FINL )
                                                 ))
