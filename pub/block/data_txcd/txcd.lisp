(module txcd_data)

;;requires ADDR_HI and ADDR_LO (for verifying that transaction init code is indeed in ROM)
;;;requires CT
(defcolumns
  NUM				;TX_NUMBER
  ADDR_HI			;ADDRESS_HIGH
  ADDR_LO			;ADDRESS_LOW
  CDS				;TXCD_SIZE
  TOT				;TXCD_COST
  RUN				;TXCD_COST_RUNNING_TOTAL
  BYTE_INDEX		;BYTE_INDEX
  LIMB_INDEX		;LIMB_INDEX
  LIMB
  (BYTE :BYTE)
  ACC
  CT
  (IS_DATA :BOOLEAN)		;IS_TRANSACTION_CALLDATA_BYTE
  )

(defconst
  G_TXDATAZERO     4
  G_TXDATANONZERO 16)

(defconstraint tx-num-init (:domain {0}) (vanishes NUM))
(defconstraint tx-num-global () (num-non-decreasing NUM))

(defconstraint index-constraint () (index-grows-or-resets NUM LIMB_INDEX))

(defconstraint forced-zero-rows ()
  (begin
   (num-zero-implies-zero NUM BYTE_INDEX)
   (num-zero-implies-zero NUM LIMB_INDEX)
   (num-zero-implies-zero NUM LIMB)
   (num-zero-implies-zero NUM ADDR_HI)
   (num-zero-implies-zero NUM ADDR_LO)
   (num-zero-implies-zero NUM CT)
   (num-zero-implies-zero NUM CDS)
   (num-zero-implies-zero NUM TOT)
   (num-zero-implies-zero NUM RUN)
   (num-zero-implies-zero NUM IS_DATA)))

;; (defconstraint stamp-constancies ()
;;              (begin
;;                  (stamp-constancy NUM ADDR_HI)
;;                  (stamp-constancy NUM ADDR_LO)
;;                  (stamp-constancy NUM TXCD_SIZE)
;;                  (stamp-constancy NUM TX_DATA_COST)))

(defconstraint gas-constraints ()
  (if-not-zero NUM
               ;; NUM[i] != NUM[i + 1]		==>		TOT[i] == RUN[i]
               (if-not-zero (remains-constant NUM)
                            (= RUN TOT))))

(defconstraint update-running-total ()
  (if-zero (didnt-change NUM)
           ;; NUM[i] == NUM[i - 1]
           (if-not-zero IS_DATA
                        (if-zero BYTE
                                 (= RUN (+ (prev RUN) G_TXDATAZERO))
                                 (= RUN (+ (prev RUN) G_TXDATANONZERO)))
                        (didnt-change RUN))
           ;; NUM[i] != NUM[i - 1]
           (if-not-zero IS_DATA
                        (if-zero BYTE
                                 (= RUN G_TXDATAZERO)
                                 (= RUN G_TXDATANONZERO))
                        (vanishes RUN))))

; we should impose that (IS_DATA == 0 at a new txn) <=> (cds == 0) <=> (tot == 0)

(defconstraint final-row (:domain {-1})
  (= RUN TOT))
