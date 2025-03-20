(module trm)

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
           (begin (vanishes! RAW_ADDRESS_HI)
                  (vanishes! RAW_ADDRESS_LO)
                  (vanishes! TRM_ADDRESS_HI)
                  (vanishes! IS_PRECOMPILE)
                  (debug (vanishes! CT))
                  (debug (vanishes! BYTE_HI))
                  (debug (vanishes! BYTE_LO)))))

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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;    2.2 stamp constancy   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP RAW_ADDRESS_HI)
         (stamp-constancy STAMP RAW_ADDRESS_LO)
         (stamp-constancy STAMP IS_PRECOMPILE)
         (stamp-constancy STAMP TRM_ADDRESS_HI)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;    2.3 PBIT constraints   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint pbit-constraint ()
  (begin (if-not-zero CT
                      (or! (remained-constant! PBIT) (did-inc! PBIT 1)))
         (if-eq CT LLARGEMO
                (eq! 1
                     (+ (shift PBIT (- 0 4))
                        (shift PBIT (- 0 3)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.4 Byte Decomposition   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint byte-decompositions ()
  (begin (byte-decomposition CT ACC_HI BYTE_HI)
         (byte-decomposition CT ACC_LO BYTE_LO)
         (byte-decomposition CT ACC_T (* BYTE_HI PBIT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    1.5 target constraints  ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint target-constraint ()
  (if-eq CT LLARGEMO
         (begin (eq! RAW_ADDRESS_HI ACC_HI)
                (eq! RAW_ADDRESS_LO ACC_LO)
                (eq! TRM_ADDRESS_HI ACC_T))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    2.4 Identifying precompiles   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint is-prec-constraint ()
  (if-eq CT LLARGEMO
         (if-zero (+ TRM_ADDRESS_HI (- RAW_ADDRESS_LO BYTE_LO))
                  (if-zero BYTE_LO
                           (vanishes! IS_PRECOMPILE)
                           (eq! (+ (* (- 9 BYTE_LO)
                                      (- (* 2 IS_PRECOMPILE) 1))
                                   (- IS_PRECOMPILE 1))
                                (reduce +
                                        (for k
                                             [0 : 7]
                                             (* (^ 2 k)
                                                (shift ONE (- 0 k)))))))
                  (vanishes! IS_PRECOMPILE))))


