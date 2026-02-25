(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;   X.4 Generalities on ROMLEX_FLAG   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   account---the-ROMLEX-lookup-requires-nonzero-code-size (:perspective account)
                 (if-zero    account/CODE_SIZE_NEW
                             (vanishes!    account/ROMLEX_FLAG)))

