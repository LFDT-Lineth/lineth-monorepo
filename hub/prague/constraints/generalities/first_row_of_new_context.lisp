(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                         ;;
;;   4.10 Setting initial parameters of new context firstRowOfNewContext   ;;
;;                                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (first-row-of-new-context kappa                         ;; row offset
                                 next-caller-context-number    ;; next caller context number
                                 next-code-fragment-index      ;; next CFI
                                 next-initial-gas )            ;; available gas in new context
  (begin
    (eq!        (shift CALLER_CONTEXT_NUMBER kappa) next-caller-context-number)
    (eq!        (shift CODE_FRAGMENT_INDEX   kappa) next-code-fragment-index  )
    (eq!        (shift GAS_EXPECTED          kappa) next-initial-gas          )
    (debug (eq! (shift CONTEXT_NUMBER        kappa) CONTEXT_NUMBER_NEW        ))
    (debug (eq! (shift CONTEXT_NUMBER        kappa) (+ 1 HUB_STAMP)           ))))
