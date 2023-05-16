(module ec_data)

(defconst
  P_HI 0x30644e72e131a029b85045b68181585d
  P_LO 0x97816a916871ca8d3c208c16d87cfd47)

(defpurefun (if-not-eq X Y Z)
  (if-not-zero (- X Y) Z))

(defunalias if-zero-else if-zero)
(defunalias doesnt-vanish is-zero)

(defpurefun (differ X Y)
  (doesnt-vanish (- X Y)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    3.2 Constancy conditions    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (stamp-consitency X)
  (if-eq (next STAMP) STAMP (remains-constant X)))

;; 3.2.1
(defconstraint stamp-constancies ()
  (begin
   (stamp-consitency TYPE)
   (stamp-consitency PCP)
   (stamp-consitency SOMETHING_WASNT_ON_G2)
   (stamp-consitency TOTAL_PAIRINGS)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    3.3 Type conditions    ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.3.1
(defconstraint exactly-one-type ()
  (if-not-zero STAMP (eq (+ EC_RECOVER EC_ADD EC_MUL EC_PAIRING) 1)))

;; 3.3.2
(defconstraint type-consistency ()
  (begin
    (if-eq EC_RECOVER 1 (= TYPE 1))
    (if-eq EC_ADD 1 (= TYPE 6))
    (if-eq EC_MUL 1 (= TYPE 7))
    (if-eq EC_PAIRING 1 (= TYPE 8))))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    3.4 Monotony    ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.4.1
(defconstraint hurdle-non-increasing ()
  (if-eq (next STAMP) STAMP 
    (if-not-eq (next HURDLE) HURDLE 
      (= (next HURDLE) (- HURDLE 1)))))

;; 3.4.2
(defconstraint notOnG2Acc-non-decreasing ()
  (if-eq (next STAMP) STAMP 
    (if-not-eq (next THIS_IS_NOT_ON_G2_ACC) THIS_IS_NOT_ON_G2_ACC 
      (= (next THIS_IS_NOT_ON_G2_ACC) (+ THIS_IS_NOT_ON_G2_ACC 1)))))

;; 3.4.3
(defconstraint notOnG2-non-decreasing ()
(if-not-zero (next INDEX)
  (if-not-eq (next THIS_IS_NOT_ON_G2) THIS_IS_NOT_ON_G2 
      (= (next THIS_IS_NOT_ON_G2) (+ THIS_IS_NOT_ON_G2 1)))))


;; 3.4.4
(defconstraint notOnG2-restarts-zero ()
(if-zero INDEX (vanishes THIS_IS_NOT_ON_G2))) 


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    3.5 Hearbeat    ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.5.1)
(defconstraint first-row (:domain {0}) (vanishes STAMP))

;; 3.5.2)
(defconstraint everything-vanish-initially ()
  (if-zero STAMP (begin
    (vanishes INDEX)
    (vanishes TYPE)
    (vanishes (+ EC_RECOVER EC_ADD EC_MUL EC_PAIRING PCP SOMETHING_WASNT_ON_G2)))))

;; 3.5.3)
(defconstraint first-index-vanishes ()
  (if-zero STAMP (vanishes (next INDEX))))

;; 3.5.4)
(defconstraint ct-min-heartbeat ()
  (if-eq-else (next STAMP) STAMP
    (if-eq-else CT_MIN 3
      (vanishes (next CT_MIN))
      (= (next CT_MIN) (+ CT_MIN 1)))
    (vanishes (next CT_MIN))))

;; 3.5.5)
(defconstraint index-heartbeat ()
(begin
  (if-eq EC_PAIRING 1
    (if-eq-else INDEX 11
      (vanishes (next INDEX))
      (= (next INDEX) (+ INDEX 1))))
  (if-eq (+ EC_ADD EC_RECOVER) 1
    (if-eq-else INDEX 7
      (vanishes (next INDEX))
      (= (next INDEX) (+ INDEX 1))))
  (if-eq EC_MUL 1
    (if-eq-else INDEX 5
      (vanishes (next INDEX))
      (= (next INDEX) (+ INDEX 1))))))

;; 3.5.6)
(defconstraint stamp-behaviour ()
  (if-not-zero (next STAMP)
    (if-zero-else (next INDEX)
      (if-eq-else EC_PAIRING 1
        (if-eq-else TOTAL_PAIRINGS (+ ACC_PAIRINGS 1)
          (differ (next STAMP) STAMP)
          (= (next STAMP) STAMP))
        (differ (next STAMP) STAMP))
      (= (next STAMP) STAMP))))

;; 3.5.7)
(defconstraint acc-pairings-behaviour ()
(if-eq-else (next STAMP) STAMP
  (if-eq-else INDEX 11
    (= (next ACC_PAIRINGS) (+ ACC_PAIRINGS 1))
    (= (next ACC_PAIRINGS) ACC_PAIRINGS))
  (vanishes (next ACC_PAIRINGS))))

;; 3.5.8)
(defconstraint finalization-constraints (:domain {-1})
(begin 
  (if-eq EC_PAIRING 1 
    (begin 
     (= INDEX 11)
     (= TOTAL_PAIRINGS (+ ACC_PAIRINGS 1))))
  (if-eq (+ EC_ADD EC_RECOVER) 1
    (= INDEX 7))
  (if-eq EC_MUL 1
    (= INDEX 5))))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    3.6 Byte decompositions    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.6.1
(defconstraint byte-decompositions ()
  (if-eq-else (next STAMP) STAMP
    (= (next ACC_DELTA) (+ (* 256 ACC_DELTA) (next BYTE_DELTA)))
    (= (next ACC_DELTA) (next BYTE_DELTA))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    3.7 Connection constraints    ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; 3.7.1
(defconstraint connection-constraints ()
  (if-not-eq (next STAMP) STAMP
    (= (next STAMP) (+ STAMP 1 (next ACC_DELTA)))))


;;;;;;;;;;;;;;;;;;;;;;
;;                  ;;
;;    3.8 Hurdle    ;;
;;                  ;;
;;;;;;;;;;;;;;;;;;;;;;


;; 3.8.1
(defconstraint final-hurdle-is-passed-to-pcp ()
  (if-not-eq (next STAMP) STAMP
    (= PCP HURDLE)))

;; 3.8.2
(defconstraint final-pcp (:domain {-1})
  (= PCP HURDLE))

