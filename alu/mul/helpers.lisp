(module mul)

(defpurefun (set-multiplication
                a_3 a_2 a_1 a_0
                b_3 b_2 b_1 b_0
                p_3 p_2 p_1 p_0
                h_3 h_2 h_1 h_0
                alpha beta eta mu)
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                (begin
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (eq!  (+ (* a_1 b_0) (* a_0 b_1))
                        (+ (* THETA2 alpha) (* THETA h_1) h_0))
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (eq!  (+ (* a_3 b_0) (* a_2 b_1) (* a_1 b_2) (* a_0 b_3))
                        (+ (* THETA2 beta) (* THETA h_3) h_2))
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (eq!  (+ (* a_0 b_0) (* THETA h_0))
                        (+ (* THETA2 eta) (* THETA p_1) p_0))
                    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (eq!  (+ eta h_1 (* THETA alpha) (* a_2 b_0) (* a_1 b_1) (* a_0 b_2) (* THETA h_2))
                        (+ (* THETA2 mu) (* THETA p_3) p_2))))


(defpurefun (prepare-lower-bound-on-two-adicity
                bytes cst bits
                x sumx y sumy ct)
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (begin
                        (running-total x sumx ct)
                        (running-total y sumy ct)
                        (if-not-zero x (vanishes! bytes))    ; (see REMARK)
                        (if-not-zero y (vanishes! bits))     ; (see REMARK)
                        (if-not-zero (- ct MMEDIUMMO) (will-remain-constant! cst))
                        (if-not-zero (- ct MMEDIUMMO)
                            (if-not-zero (- 1 x)            ; (see REMARK)
                                (if-not-zero (next x)       ; (see REMARK)
                                    (eq! cst bytes))))
                        (if-eq ct MMEDIUMMO
                            (begin
                                (if-not-zero (- 1 x)        ; (see REMARK)
                                    (eq! cst bytes))
                                (eq! cst (bit-decomposition-of-byte bits))))))
;; REMARK:
;; within the scope of prepare-lower-bound-on-two-adicity
;; the running-total applies so that x and y are forced to
;; be binary (if only for the counter-cycle where the above applies)
;; in any case: 1 - x != 0 <=> x = 0

(defpurefun (bit-decomposition-of-byte bits)
                (+  (* 128  (shift bits -7))
                    (* 64   (shift bits -6))
                    (* 32   (shift bits -5))
                    (* 16   (shift bits -4))
                    (* 8    (shift bits -3))
                    (* 4    (shift bits -2))
                    (* 2    (shift bits -1))
                    bits))

(defpurefun (running-total x sumx ct)
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                    (begin
                        (is-binary x)
                        (if-zero ct
                            (begin
                                (vanishes! x)
                                (vanishes! sumx)))
                        (if-not-zero (- ct MMEDIUMMO)
                            (begin
                             (or! (will-remain-constant! x) (will-inc! x 1))
                             (will-eq! sumx (+ sumx (next x)))))))
