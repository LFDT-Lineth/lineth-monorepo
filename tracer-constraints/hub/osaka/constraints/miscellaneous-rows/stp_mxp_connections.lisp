(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   9.5.1 STP <> MXP connections   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; optional ...
(defconstraint   STP-MXP-connections  (:guard  (*  PEEK_AT_MISCELLANEOUS   misc/STP_FLAG))
                 (begin
                   (eq!   misc/MXP_FLAG      1)
                   (eq!   misc/STP_GAS_MXP   misc/MXP_GAS_MXP)))
