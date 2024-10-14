(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                    ;;
;;   4.X Stack height columns HEIGHT and HEIGHT_NEW   ;;
;;                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (stack-exceptions)    (force-bool (+ stack/SUX stack/SOX)))

;; TODO: this should be debug!
(defconstraint    generalities---stack-height---automatic-vanishing   ()
                  (if-zero    TX_EXEC
                              (begin
                                (vanishes!    HEIGHT)
                                (vanishes!    HEIGHT_NEW))))

(defconstraint    generalities---stack-height---update   (:perspective stack)
                  (if-not-zero    (stack-exceptions)
                                  (vanishes!    HEIGHT_NEW)
                                  (eq!          HEIGHT_NEW    (+ (- HEIGHT stack/DELTA) stack/ALPHA))))
