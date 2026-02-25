(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   X.1.11 Precompile flag related   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (account-isnt-precompile   kappa) (vanishes!   (shift    account/IS_PRECOMPILE    kappa)))
(defun (account-is-precompile     kappa) (eq!         (shift    account/IS_PRECOMPILE    kappa) 1))


