(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;   X.Y.Z ROM_LEX module triggering   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun   (account-dont-trigger-ROM_LEX            relof)       (eq!   (shift   account/ROMLEX_FLAG   relof)   0))
(defun   (account-do-trigger-ROM_LEX              relof)       (eq!   (shift   account/ROMLEX_FLAG   relof)   1))
(defun   (account-conditionally-trigger-ROM_LEX   relof
                                                  condition)   (eq!   (shift   account/ROMLEX_FLAG   relof)   condition))
