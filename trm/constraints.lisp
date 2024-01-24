(module trm)

(defconst 
  LLARGEMO 15)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; 1
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

;; 3
(defconstraint null-stamp-null-columns ()
  (if-zero STAMP
           (begin (vanishes! ADDR_HI)
                  (vanishes! ADDR_LO)
                  (vanishes! TRM_ADDR_HI)
                  (vanishes! IS_PREC)
                  (vanishes! CT)
                  (vanishes! BYTE_HI)
                  (vanishes! BYTE_LO))))

(defconstraint heartbeat ()
  (begin  ;; 2
         (or! (will-remain-constant! STAMP) (will-inc! STAMP 1))
         ;; 4
         (if-not-zero (- (next STAMP) STAMP)
                      (vanishes! (next CT)))
         ;; 5
         (if-not-zero STAMP
                      (if-eq-else CT LLARGEMO (will-inc! STAMP 1) (will-inc! CT 1)))))

;; 6
(defconstraint last-row (:domain {-1})
  (if-not-zero STAMP
               (eq! CT LLARGEMO)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.2 stamp constancy    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP ADDR_HI)
         (stamp-constancy STAMP ADDR_LO)
         (stamp-constancy STAMP IS_PREC)
         (stamp-constancy STAMP TRM_ADDR_HI)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    2.3 PBIT constraints   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint pbit-constraint ()
  (begin (if-zero CT
                  (vanishes! PBIT)
                  (or! (remained-constant! PBIT) (did-inc! PBIT 1)))
         (if-eq CT 12
                (begin (vanishes! (prev PBIT))
                       (eq! PBIT 1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;    2.4 Byte Decomposition   ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint byte-decompositions ()
  (begin (byte-decomposition CT ACC_HI BYTE_HI)
         (byte-decomposition CT ACC_LO BYTE_LO)
         (byte-decomposition CT ACC_T (* BYTE_HI PBIT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    1.5 target constraints    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;)
(defconstraint target-constraint ()
  (if-eq CT 15
         (begin (eq! ADDR_HI ACC_HI)
                (eq! ADDR_LO ACC_LO)
                (eq! TRM_ADDR_HI ACC_T))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    2.4 Identifying precompiles   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint is-prec-constraint ()
  (if-eq CT 15
         (if-zero (+ TRM_ADDR_HI (- ADDR_LO BYTE_LO))
                  (if-zero BYTE_LO
                           (vanishes! IS_PREC)
                           (eq! (+ (* (- 9 BYTE_LO)
                                      (- (* 2 IS_PREC) 1))
                                   (- IS_PREC 1))
                                (reduce +
                                        (for k
                                             [0 : 7]
                                             (* (^ 2 k)
                                                (shift ONE (- 0 k)))))))
                  (vanishes! IS_PREC))))


