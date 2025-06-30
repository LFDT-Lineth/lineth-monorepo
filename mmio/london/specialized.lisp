(module mmio)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;  Specialized constraints  ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;Plateau
(defpurefun (plateau x cst counter)
  (if-zero cst
           (eq! x 1)
           (if-zero counter
                    (vanishes! x)
                    (if-eq-else counter cst (eq! x 1) (remained-constant! x)))))

;Power
(defpurefun (power pow x counter)
  (if-zero counter
           (if-zero x
                    (eq! pow 1)
                    (eq! pow 256))
           (if-zero x
                    (remained-constant! pow)
                    (eq! pow
                         (* (prev pow) 256)))))

;AntiPower
(defpurefun (antipower pow x counter)
  (if-zero counter
           (if-zero x
                    (eq! pow 256)
                    (eq! pow 1))
           (if-zero x
                    (eq! pow
                         (* (prev pow) 256))
                    (remained-constant! pow))))

;IsolateSuffix
(defpurefun (isolate-suffix accumulator byte x counter)
  (if-zero counter
           (if-zero x
                    (vanishes! accumulator)
                    (eq! accumulator byte))
           (if-zero x
                    (remained-constant! accumulator)
                    (eq! accumulator
                         (+ (* 256 (prev accumulator))
                            byte)))))

;IsolatePrefix
(defpurefun (isolate-prefix accumulator byte x counter)
  (if-zero counter
           (if-zero x
                    (eq! accumulator byte)
                    (vanishes! accumulator))
           (if-zero x
                    (eq! accumulator
                         (+ (* 256 (prev accumulator))
                            byte))
                    (remained-constant! accumulator))))

;IsolateChunk
(defpurefun (isolate-chunk accumulator byte x y counter)
  (if-zero counter
           (if-zero x
                    (vanishes! accumulator)
                    (eq! accumulator byte))
           (if-zero x
                    (vanishes! accumulator)
                    (if-zero y
                             (eq! accumulator
                                  (+ (* 256 (prev accumulator))
                                     byte))
                             (remained-constant! accumulator)))))
