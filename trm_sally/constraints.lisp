(module trm)

(defconst
  MAX_PREC_ADDR 9
  LLARGEMO     15)


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0})
               (vanishes! STAMP))

(defconstraint heartbeat ()
               (begin
                 (* (will-remain-constant! STAMP) (will-inc! STAMP 1))
                 (if-not-zero (will-remain-constant! STAMP) (vanishes! (next CT)))
                 (if-zero STAMP
                          (begin
                            (vanishes! ADDR_HI)
                            (vanishes! ADDR_LO)
                            (vanishes! TRM_ADDR_HI)
                            (vanishes! IS_PREC)
                            (vanishes! CT)
                            (vanishes! BYTE_HI)
                            (vanishes! BYTE_LO)))
                 (if-not-zero STAMP
                              (if-eq-else CT LLARGEMO
                                          (will-inc! STAMP 1)
                                          (will-inc! CT 1)))))

(defconstraint last-row (:domain {-1})
               (if-not-zero STAMP  (= CT LLARGEMO)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    2.2 stamp constancy    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint stamp-constancies ()
               (begin
                 (stamp-constancy STAMP ADDR_HI)
                 (stamp-constancy STAMP ADDR_LO)
                 (stamp-constancy STAMP TRM_ADDR_HI)
                 (stamp-constancy STAMP IS_PREC)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    2.3 Pivot bit constraints    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint pivot-bit ()
               (begin
                 (is-binary PBIT)
                 (if-zero CT
                          (vanishes! PBIT))
                 (if-not-zero CT 
                              (vanishes! (* (remained-constant! PBIT) (did-inc! PBIT 1))))
                 (if-eq CT 12
                        (= 1 (+ PBIT (prev PBIT))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;    2.4 binary, bytehood and byte decompositions   ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint binary-and-byte-decompositions ()
               (begin
                 (byte-decomposition CT ACC_HI BYTE_HI)
                 (byte-decomposition CT ACC_LO BYTE_LO)
                 (byte-decomposition CT ACC_T (* PBIT BYTE_HI))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    2.5 Target constraints       ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint target-constraints ()
               (if-eq CT LLARGEMO
                      (begin 
                        (= ADDR_HI ACC_HI)
                        (= ADDR_LO ACC_LO)
                        (= TRM_ADDR_HI ACC_T))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;    2.6 Identifying precompiles    ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (bit-decomposition-associated-with-ONES) (+ ONES
                                                   (* (shift ONES -1) 2)
                                                   (* (shift ONES -2) 4)
                                                   (* (shift ONES -3) 8)
                                                   (* (shift ONES -4) 16)
                                                   (* (shift ONES -5) 32)
                                                   (* (shift ONES -6) 64)
                                                   (* (shift ONES -7) 128)))

(defconstraint binaryness ()
               (begin
                 (is-binary IS_PREC)
                 (is-binary ONES)))

(defconstraint identifying-precompiles ()
               (if-eq CT LLARGEMO
                      (begin 
                        (if-not-zero (+ TRM_ADDR_HI (- ADDR_LO BYTE_LO))
                                     (= IS_PREC 0))
                        (if-zero (+ TRM_ADDR_HI (- ADDR_LO BYTE_LO))
                                 (if-zero BYTE_LO
                                          (= IS_PREC 0)
                                          (= 
                                            (+ (* (- MAX_PREC_ADDR BYTE_LO) (- (* 2 IS_PREC) 1) (- IS_PREC 1)))
                                            (bit-decomposition-associated-with-ONES)))))))
