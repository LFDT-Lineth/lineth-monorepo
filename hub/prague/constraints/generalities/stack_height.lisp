(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;   4.X Stack height columns HEIGHT and HEIGHT_NEW   ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (stack-exceptions)    (force-bin (+ stack/SUX stack/SOX)))

(defconstraint    generalities---stack-height---hub-stamp-constancies   ()
                  (begin
                    (hub-stamp-constancy    HEIGHT)
                    (hub-stamp-constancy    HEIGHT_NEW)))

;; ;; This is debug!
;; (defconstraint    generalities---stack-height---automatic-vanishing   ()
;;                   (if-zero    TX_EXEC
;;                               (begin
;;                                 (vanishes!    HEIGHT)
;;                                 (vanishes!    HEIGHT_NEW))))

(defconstraint    generalities---stack-height---update   (:perspective stack)
                  (if-not-zero    (stack-exceptions)
                                  ;; stack exception ≡ true
                                  (vanishes!    HEIGHT_NEW)
                                  ;; stack exception ≡ false
                                  (eq!          HEIGHT_NEW    (+ (- HEIGHT stack/DELTA) stack/ALPHA))))
