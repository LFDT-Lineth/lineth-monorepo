(module hub)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   X.Y Transaction phase flags   ;;
;;   X.Y.Z Shorthands              ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (phase-flag-sum)    (force-bin   (+    TX_SKIP         TX_WARM          TX_INIT          TX_EXEC           TX_FINL)))
(defun    (phase-wght-sum)                 (+    TX_SKIP    (* 2 TX_WARM)    (* 4 TX_INIT)    (* 8 TX_EXEC)    (* 16 TX_FINL)))
