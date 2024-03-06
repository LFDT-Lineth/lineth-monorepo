(module gas)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    3.1 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;; 1
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

;; 2
(defconstraint stamp-vanishing-values ()
  (if-zero STAMP
           (begin (vanishes! CT)
                  (vanishes! [BYTE 1])
                  (vanishes! [BYTE 2])
                  (vanishes! GAS_ACTL)
                  (vanishes! GAS_COST)
                  (vanishes! OOGX))))

;; 3
(defconstraint stamp-increments ()
  (any! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

;; 4
(defconstraint counter-reset ()
  (if-not-zero (will-remain-constant! STAMP)
               (vanishes! (next CT))))

;; 5
(defconstraint instruction-counter-cycle ()
  (if-not-zero STAMP
               (if-eq-else CT 7 (will-inc! STAMP 1) (will-inc! CT 1))))

;; 6
(defconstraint final-row (:domain {-1})
  (if-not-zero STAMP
               (eq! CT 7)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.2 counter constancy    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint counter-constancy ()
  (begin (counter-constancy CT GAS_ACTL)
         (counter-constancy CT GAS_COST)
         (counter-constancy CT OOGX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    3.3 Byte decompositions    ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint byte-decompositions ()
  (for k [1:2] (byte-decomposition CT [ACC k] [BYTE k])))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    3.3 Target constraints     ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target-1 ()
  (if-eq CT 7
         (begin (eq! [ACC 1] GAS_ACTL)
                (eq! [ACC 2]
                     (- (* (- (* 2 OOGX) 1)
                           (- GAS_COST GAS_ACTL))
                        OOGX)))))


