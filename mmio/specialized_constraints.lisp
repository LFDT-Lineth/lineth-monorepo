(module mmio)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                           ;;
;;              3.4 Binary plateau constraints               ;;
;;                                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;Plateau
(defun (plateau X C CT)
                (if-zero-else C
                    (eq X 1)
                    (if-zero-else CT
                        (vanishes! X)
                        (if-eq-else CT C
                            (eq X (+ (prev X) 1))
                            (eq X (prev X))))))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;              3.5 Power constraints               ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;Power
(defun (power P X CT)
                (if-zero-else MICRO_STAMP
                    (vanishes! P)
                    (if-zero-else FAST
                        (begin
                            (if-zero-else CT
                                (eq P 1)
                                (if-zero-else X
                                    (eq P (shift P -1))
                                    (eq P (* 256 (shift P -1))))))
                        (vanishes! P))))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;              3.6 Suffix extraction               ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;IsolateSuffix
(defun (isolate-suffix ACC B X CT)
                (if-zero-else CT
                    (if-zero-else X
                        (vanishes! ACC)
                        (eq ACC B))
                    (if-zero-else X
                        (eq ACC (shift ACC -1))
                        (eq ACC (+  (* 256 (shift ACC -1)) B)))))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;              3.7 Prefix extraction               ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;IsolatePrefix
(defun (isolate-prefix ACC B X CT)
                (if-zero-else CT
                    (if-zero-else X
                        (eq ACC B)
                        (vanishes! ACC))
                    (if-zero-else X
                        (eq ACC (+  (* 256 (shift ACC -1)) B))
                        (eq ACC (shift ACC -1)))))




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;              3.8 Chunk extraction               ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;IsolateChunk
(defun (isolate-chunk ACC B X Y CT)
                (if-zero-else CT
                    (if-zero-else X
                        (vanishes! ACC)
                        (eq ACC B))
                    (if-zero-else X
                        (vanishes! ACC)
                        (if-zero-else Y
                            (eq ACC (+ (* 256 (shift ACC -1)) B))
                            (eq ACC (shift ACC -1))))))
