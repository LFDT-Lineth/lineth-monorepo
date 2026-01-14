(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;   X.Y System flags   ;;
;;   X.Y.Z Shorthands   ;;
;;                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (system-flag-sum)    (force-bin   (+    SYSI         USER          SYSF)))
(defun    (system-wght-sum)                 (+    SYSI    (* 2 USER)    (* 4 SYSF)))
