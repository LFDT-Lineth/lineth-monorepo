(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   X.1.10 Code fragment index and trimming   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (account-retrieve-code-fragment-index    kappa) (eq! (shift account/ROMLEX_FLAG   kappa) 1))

(defun (account-trim-address   kappa                   ;; row offset
                               raw-address-hi          ;; high part of raw, potentially untrimmed address
                               raw-address-lo          ;; low  part of raw, potentially untrimmed address
                               ) (begin
                               (eq! (shift   account/TRM_FLAG             kappa) 1)
                               (eq! (shift   account/TRM_RAW_ADDRESS_HI   kappa) raw-address-hi)
                               (eq! (shift   account/ADDRESS_LO           kappa) raw-address-lo)))


