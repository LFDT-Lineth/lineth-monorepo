(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    X.Y.Z perspective transitions    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defproperty    perspective-transitions---sanitychecks
                (if-not-zero    (-   (next   TOTL_TXN_NUMBER)   TOTL_TXN_NUMBER)
                                (begin
                                  (will-eq!       HUB    1)
                                  (if-not-zero    TOTL_TXN_NUMBER
                                                  (eq!    CMPTN    1)))))

(defconstraint    perspective-transitions---computation-rows-lead-to-computation-rows-within-CT-cycles   ()
                  (if-not-zero    (-   CT_MAX   CT)
                                  (if-not-zero    CMPTN
                                                  (will-eq!    CMPTN    1))))
