(module trm)

(defconst
  BALANCE      0x31
  EXTCODESIZE  0x3b
  EXTCODECOPY  0x3c
  EXTCODEHASH  0x3f
  CALL         0xf1
  CALLCODE     0xf2
  DELEGATECALL 0xf4
  STATICCALL   0xfa
  SELFDESTRUCT 0xff
  LLARGEMO     15
  THETA        340282366920938463463374607431768211456) ;; note that 340282366920938463463374607431768211456 = 256^16


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    1.3 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0})
                (vanishes STAMP))

(defconstraint heartbeat ()
  (begin
   (* (remains-constant STAMP) (inc STAMP 1))
   (if-not-zero (remains-constant STAMP) (vanishes (next CT)))
   (if-not-zero STAMP
                (if-eq-else CT LLARGEMO
                            (inc STAMP 1)
                            (inc CT 1)))))

(defconstraint last-row (:domain {-1})
  (if-not-zero STAMP  (= CT LLARGEMO)))

(defconstraint stamp-constancies ()
  (begin
   (stamp-constancy STAMP ADDR_HI)
   (stamp-constancy STAMP ADDR_LO)
   (stamp-constancy STAMP IS_PREC_HI)
   (stamp-constancy STAMP TRM_ADDR_HI)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;    1.4 binary, bytehood and byte decompositions   ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint binary-and-byte-decompositions ()
  (begin
   (is-binary PBIT)
   (is-binary IS_PREC)
   (is-binary SPECIAL_ONE)
   (byte-decomposition CT ACC_HI BYTE_HI)
   (byte-decomposition CT ACC_LO BYTE_LO)))

;TODO: bytehood constraints


;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    1.5 constraints    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
)
