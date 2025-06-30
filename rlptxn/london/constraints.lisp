(module rlptxn)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    2.3 Global Constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.3.1 Constancy columns  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Definition counter-incrementing.
(defpurefun (counter-incrementing CT C)
  (if-not-zero CT
               (or! (remained-constant! C) (did-inc! C 1))))

;; Definition phase-constancy.
(defpurefun (phase-constancy phase column)
  (if-eq (* phase (prev phase)) 1
         (remained-constant! column)))

;; Definition phase-incrementing
(defpurefun (phase-incrementing phase C)
  (if-eq (* phase (prev phase)) 1
         (or! (remained-constant! C) (did-inc! C 1))))

;; Definition phase-decrementing
(defpurefun (phase-decrementing phase C)
  (if-eq (* phase (prev phase)) 1
         (or! (remained-constant! C) (did-dec! C 1))))

;; Definition block-constant
(defpurefun (block-constant ABS_TX_NUM C)
  (if-zero ABS_TX_NUM
           (vanishes! C)
           (will-remain-constant! C)))

;; 2.3.1.1
(defconstraint stamp-constancies ()
  (begin (stamp-constancy ABS_TX_NUM TYPE)
         (stamp-constancy ABS_TX_NUM REQUIRES_EVM_EXECUTION)
         (stamp-constancy ABS_TX_NUM CODE_FRAGMENT_INDEX)))

;; 2.3.1.2
(defconstraint counter-constancy ()
  (begin (counter-constancy CT [INPUT 1])
         (counter-constancy CT [INPUT 2])
         (counter-constancy CT nSTEP)
         (counter-constancy CT LT)
         (counter-constancy CT LX)
         (counter-constancy CT IS_PREFIX)
         (counter-constancy CT nADDR)
         (counter-constancy CT nKEYS)
         (counter-constancy CT nKEYS_PER_ADDR)
         (counter-constancy CT [DEPTH 1])
         (counter-constancy CT [DEPTH 2])))

(defconstraint counter-incrementing ()
  (counter-incrementing CT LC_CORRECTION))

(defconstraint counter-incrementing-except-data-prefix ()
  (if-zero (* IS_PHASE_DATA IS_PREFIX)
           (counter-incrementing CT LIMB_CONSTRUCTED)))

(defconstraint phaseRlpPrefix-constancy ()
  (begin (phase-constancy IS_PHASE_RLP_PREFIX RLP_LT_BYTESIZE)
         (phase-constancy IS_PHASE_RLP_PREFIX RLP_LX_BYTESIZE)
         (phase-constancy IS_PHASE_RLP_PREFIX DATA_HI)
         (phase-constancy IS_PHASE_RLP_PREFIX DATA_LO)))

(defconstraint phaseData-decrementing ()
  (phase-decrementing IS_PHASE_DATA IS_PREFIX))

(defconstraint phasek-constancies ()
  (begin (phase-constancy IS_PHASE_NONCE DATA_HI)
         (phase-constancy IS_PHASE_NONCE DATA_LO)
         (phase-constancy IS_PHASE_GAS_PRICE DATA_HI)
         (phase-constancy IS_PHASE_GAS_PRICE DATA_LO)
         (phase-constancy IS_PHASE_MAX_PRIORITY_FEE_PER_GAS DATA_HI)
         (phase-constancy IS_PHASE_MAX_PRIORITY_FEE_PER_GAS DATA_LO)
         (phase-constancy IS_PHASE_MAX_FEE_PER_GAS DATA_HI)
         (phase-constancy IS_PHASE_MAX_FEE_PER_GAS DATA_LO)
         (phase-constancy IS_PHASE_GAS_LIMIT DATA_HI)
         (phase-constancy IS_PHASE_GAS_LIMIT DATA_LO)
         (phase-constancy IS_PHASE_TO DATA_HI)
         (phase-constancy IS_PHASE_TO DATA_LO)
         (phase-constancy IS_PHASE_VALUE DATA_HI)
         (phase-constancy IS_PHASE_VALUE DATA_LO)
         (phase-constancy IS_PHASE_DATA DATA_HI)
         (phase-constancy IS_PHASE_DATA DATA_LO)
         (phase-constancy IS_PHASE_ACCESS_LIST DATA_HI)
         (phase-constancy IS_PHASE_ACCESS_LIST DATA_LO)))

(defconstraint block-constancies ()
  (block-constant ABS_TX_NUM ABS_TX_NUM_INFINY))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    2.3.2 Global Phase Constraints    ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (weighted-flag-sum)
  (+ (* IS_PHASE_RLP_PREFIX RLP_TXN_PHASE_RLP_PREFIX)
     (* IS_PHASE_CHAIN_ID RLP_TXN_PHASE_CHAIN_ID)
     (* IS_PHASE_NONCE RLP_TXN_PHASE_NONCE)
     (* IS_PHASE_GAS_PRICE RLP_TXN_PHASE_GAS_PRICE)
     (* IS_PHASE_MAX_PRIORITY_FEE_PER_GAS RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS)
     (* IS_PHASE_MAX_FEE_PER_GAS RLP_TXN_PHASE_MAX_FEE_PER_GAS)
     (* IS_PHASE_GAS_LIMIT RLP_TXN_PHASE_GAS_LIMIT)
     (* IS_PHASE_TO RLP_TXN_PHASE_TO)
     (* IS_PHASE_VALUE RLP_TXN_PHASE_VALUE)
     (* IS_PHASE_DATA RLP_TXN_PHASE_DATA)
     (* IS_PHASE_ACCESS_LIST RLP_TXN_PHASE_ACCESS_LIST)
     (* IS_PHASE_BETA RLP_TXN_PHASE_BETA)
     (* IS_PHASE_Y RLP_TXN_PHASE_Y)
     (* IS_PHASE_R RLP_TXN_PHASE_R)
     (* IS_PHASE_S RLP_TXN_PHASE_S)))

(defconstraint phase-id-to-phase-flag ()
  (eq! PHASE (weighted-flag-sum)))

;; 2.3.2.1
(defconstraint initial-stamp (:domain {0})
  (vanishes! ABS_TX_NUM))

;; 2.3.2.2
(defun (flag-sum)
  (force-bin (+ IS_PHASE_RLP_PREFIX
                IS_PHASE_CHAIN_ID
                IS_PHASE_NONCE
                IS_PHASE_GAS_PRICE
                IS_PHASE_MAX_PRIORITY_FEE_PER_GAS
                IS_PHASE_MAX_FEE_PER_GAS
                IS_PHASE_GAS_LIMIT
                IS_PHASE_TO
                IS_PHASE_VALUE
                IS_PHASE_DATA
                IS_PHASE_ACCESS_LIST
                IS_PHASE_BETA
                IS_PHASE_Y
                IS_PHASE_R
                IS_PHASE_S)))

(defconstraint flag-sum-is-one-or-padding ()
  (eq! (~ ABS_TX_NUM) (flag-sum)))

;; 2.3.2.4
(defconstraint ABS_TX_NUM-evolution ()
  (if (or! (eq! IS_PHASE_RLP_PREFIX 0) (remained-constant! IS_PHASE_RLP_PREFIX))
           ;; no change
           (remained-constant! ABS_TX_NUM)
           ;; increment
           (did-inc! ABS_TX_NUM 1)))

(defconstraint set-to-hash-by-prover-flag ()
  (eq! TO_HASH_BY_PROVER (* LC LX)))

;; 2.3.2.6
(defun (flag-sum-lt-and-lx-are-one)
  (force-bin (+ IS_PHASE_CHAIN_ID
                 IS_PHASE_NONCE
                 IS_PHASE_GAS_PRICE
                 IS_PHASE_MAX_PRIORITY_FEE_PER_GAS
                 IS_PHASE_MAX_FEE_PER_GAS
                 IS_PHASE_GAS_LIMIT
                 IS_PHASE_TO
                 IS_PHASE_VALUE
                 IS_PHASE_DATA
                 IS_PHASE_ACCESS_LIST)))

(defconstraint LT-and-LX ()
  (if-eq (flag-sum-lt-and-lx-are-one) 1
         (begin (eq! LT 1)
                (eq! LX 1))))

;; 2.3.2.7
(defun (flag-sum-only-lt)
  (force-bin (+ IS_PHASE_Y IS_PHASE_R IS_PHASE_S)))

(defconstraint LT-only ()
  (if-eq (flag-sum-only-lt) 1
         (begin (eq! 1 LT)
                (vanishes! LX))))

;; 2.3.2.8
(defconstraint no-done-no-end ()
  (if-zero DONE
           (vanishes! PHASE_END)))

;; 2.3.2.9
(defun (weighted-diff-flag-sum)
  (+ (* (- (next IS_PHASE_RLP_PREFIX) IS_PHASE_RLP_PREFIX)
        RLP_TXN_PHASE_RLP_PREFIX)
     (* (- (next IS_PHASE_CHAIN_ID) IS_PHASE_CHAIN_ID)
        RLP_TXN_PHASE_CHAIN_ID)
     (* (- (next IS_PHASE_NONCE) IS_PHASE_NONCE)
        RLP_TXN_PHASE_NONCE)
     (* (- (next IS_PHASE_GAS_PRICE) IS_PHASE_GAS_PRICE)
        RLP_TXN_PHASE_GAS_PRICE)
     (* (- (next IS_PHASE_MAX_PRIORITY_FEE_PER_GAS) IS_PHASE_MAX_PRIORITY_FEE_PER_GAS)
        RLP_TXN_PHASE_MAX_PRIORITY_FEE_PER_GAS)
     (* (- (next IS_PHASE_MAX_FEE_PER_GAS) IS_PHASE_MAX_FEE_PER_GAS)
        RLP_TXN_PHASE_MAX_FEE_PER_GAS)
     (* (- (next IS_PHASE_GAS_LIMIT) IS_PHASE_GAS_LIMIT)
        RLP_TXN_PHASE_GAS_LIMIT)
     (* (- (next IS_PHASE_TO) IS_PHASE_TO)
        RLP_TXN_PHASE_TO)
     (* (- (next IS_PHASE_VALUE) IS_PHASE_VALUE)
        RLP_TXN_PHASE_VALUE)
     (* (- (next IS_PHASE_DATA) IS_PHASE_DATA)
        RLP_TXN_PHASE_DATA)
     (* (- (next IS_PHASE_ACCESS_LIST) IS_PHASE_ACCESS_LIST)
        RLP_TXN_PHASE_ACCESS_LIST)
     (* (- (next IS_PHASE_BETA) IS_PHASE_BETA)
        RLP_TXN_PHASE_BETA)
     (* (- (next IS_PHASE_Y) IS_PHASE_Y)
        RLP_TXN_PHASE_Y)
     (* (- (next IS_PHASE_R) IS_PHASE_R)
        RLP_TXN_PHASE_R)
     (* (- (next IS_PHASE_S) IS_PHASE_S)
        RLP_TXN_PHASE_S)))

(defconstraint no-end-no-changephase (:guard ABS_TX_NUM)
  (if-zero PHASE_END
           (vanishes! (weighted-diff-flag-sum))))

;; 2.3.2.10
(defconstraint phase-transition ()
  (if-eq PHASE_END 1
         (begin (if-eq IS_PHASE_RLP_PREFIX 1
                       (if-zero TYPE
                                (eq! (next IS_PHASE_NONCE) 1)
                                (eq! (next IS_PHASE_CHAIN_ID) 1)))
                (if-eq IS_PHASE_CHAIN_ID 1
                       (eq! (next IS_PHASE_NONCE) 1))
                (if-eq IS_PHASE_NONCE 1
                       (if-eq-else TYPE 2
                                   (eq! (next IS_PHASE_MAX_PRIORITY_FEE_PER_GAS) 1)
                                   (eq! (next IS_PHASE_GAS_PRICE) 1)))
                (if-eq IS_PHASE_GAS_PRICE 1
                       (eq! (next IS_PHASE_GAS_LIMIT) 1))
                (if-eq IS_PHASE_MAX_PRIORITY_FEE_PER_GAS 1
                       (eq! (next IS_PHASE_MAX_FEE_PER_GAS) 1))
                (if-eq IS_PHASE_MAX_FEE_PER_GAS 1
                       (eq! (next IS_PHASE_GAS_LIMIT) 1))
                (if-eq IS_PHASE_GAS_LIMIT 1
                       (eq! (next IS_PHASE_TO) 1))
                (if-eq IS_PHASE_TO 1
                       (eq! (next IS_PHASE_VALUE) 1))
                (if-eq IS_PHASE_VALUE 1
                       (eq! (next IS_PHASE_DATA) 1))
                (if-eq IS_PHASE_DATA 1
                       (begin ;;(debug (vanishes! RLP_TXN_PHASE_SIZE))
                              (vanishes! DATA_GAS_COST)
                              (if-zero TYPE
                                       (eq! (next IS_PHASE_BETA) 1)
                                       (eq! (next IS_PHASE_ACCESS_LIST) 1))))
                (if-eq IS_PHASE_ACCESS_LIST 1
                       (begin ;;(debug (vanishes! RLP_TXN_PHASE_SIZE))
                              (vanishes! nADDR)
                              (vanishes! nKEYS)
                              (vanishes! nKEYS_PER_ADDR)
                              (eq! (next IS_PHASE_Y) 1)))
                (if-eq IS_PHASE_BETA 1
                       (eq! (next IS_PHASE_R) 1))
                (if-eq IS_PHASE_Y 1
                       (eq! (next IS_PHASE_R) 1))
                (if-eq IS_PHASE_R 1
                       (eq! (next IS_PHASE_S) 1))
                (if-eq IS_PHASE_S 1
                       (begin (vanishes! RLP_LT_BYTESIZE)
                              (vanishes! RLP_LX_BYTESIZE)
                              (eq! (next IS_PHASE_RLP_PREFIX) 1))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.3.3 Byte decomposition's loop heartbeat  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; 2.3.3.2 & 2.3.3.3
(defconstraint ct-imply-done (:guard ABS_TX_NUM)
  (if-eq-else CT (- nSTEP 1) (eq! DONE 1) (vanishes! DONE)))

;; 2.3.3.4 & 2.3.3.5
(defconstraint done-imply-heartbeat (:guard ABS_TX_NUM)
  (if-zero DONE
           (will-inc! CT 1)
           (begin (eq! LC (- 1 LC_CORRECTION))
                  (vanishes! (next CT)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.3.4 Blind Byte and Bit decomposition  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; 2.3.4.1
(defconstraint byte-decompositions ()
  (for k [1:2] (byte-decomposition CT [ACC k] [BYTE k])))

;; 2.3.4.2
(defconstraint bit-decomposition ()
  (if-zero CT
           (eq! BIT_ACC BIT)
           (eq! BIT_ACC
                (+ (* 2 (prev BIT_ACC))
                   BIT))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.3.5 Global Constraints  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; 2.3.5.1 & 2.3.5.2
(defconstraint init-index ()
  (if (remained-constant! ABS_TX_NUM)
      (begin (eq! INDEX_LT
                   (+ (prev INDEX_LT)
                      (* (prev LC) (prev LT))))
             (eq! INDEX_LX
                   (+ (prev INDEX_LX)
                      (* (prev LC) (prev LX)))))
      (begin (vanishes! INDEX_LT)
             (vanishes! INDEX_LX))))

;; 2.3.5.3
(defun (flag-sum-wo-phase-rlp-prefix)
  (force-bin (- (flag-sum) IS_PHASE_RLP_PREFIX)))

(defconstraint rlpbytesize-decreasing ()
  (if-eq (flag-sum-wo-phase-rlp-prefix) 1
         (begin (eq! RLP_LT_BYTESIZE
                     (- (prev RLP_LT_BYTESIZE) (* LC LT nBYTES)))
                (eq! RLP_LX_BYTESIZE
                     (- (prev RLP_LX_BYTESIZE) (* LC LX nBYTES))))))

(defconstraint lc-correction-nullity ()
  (if-zero (+ IS_PHASE_RLP_PREFIX IS_PHASE_DATA IS_PHASE_BETA)
           (vanishes! LC_CORRECTION)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    2.3.6 Finalisation Constraints    ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint finalisation (:domain {-1})
  (if-not-zero ABS_TX_NUM
               (begin (eq! ABS_TX_NUM_INFINY ABS_TX_NUM)
                      (eq! 1 PHASE_END)
                      (eq! 1 IS_PHASE_S))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3 Constraints patterns   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.1 RLP prefix constraint of a 32 bytes integer  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defpurefun (rlpPrefixLongInt input_hi
                              input_lo
                              ct
                              nStep
                              done
                              byte_hi
                              byte_lo
                              acc_hi
                              acc_lo
                              byteSize
                              power
                              bit
                              bitAcc
                              limb
                              lc
                              nBytes)
  (begin (if-zero input_hi
                  (byteCountAndPower ct nStep done acc_lo byteSize power)
                  (byteCountAndPower ct nStep done acc_hi byteSize power))
         (if-eq done 1
                (begin (eq! acc_hi input_hi)
                       (eq! acc_lo input_lo)
                       (if-zero input_hi
                                (begin (eq! bitAcc byte_lo)
                                       (if-zero (+ (shift bit -7) (- byteSize 1))
                                                (begin (vanishes! (prev lc))
                                                       (eq! limb (* input_lo power))
                                                       (eq! nBytes byteSize))
                                                (begin (eq! 1
                                                            (+ (shift lc -2) (prev lc)))
                                                       (eq! (prev limb)
                                                            (* (+ RLP_PREFIX_INT_SHORT byteSize)
                                                               (^ 256 LLARGEMO)))
                                                       (eq! (prev nBytes) 1)
                                                       (eq! limb (* input_lo power))
                                                       (eq! nBytes byteSize))))
                                (begin (eq! (+ (shift lc -3) (shift lc -2))
                                            1)
                                       (eq! (shift limb -2)
                                            (* (+ RLP_PREFIX_INT_SHORT LLARGE byteSize) (^ 256 LLARGEMO)))
                                       (eq! (shift nBytes -2) 1)
                                       (eq! (prev limb) (* input_hi power))
                                       (eq! (prev nBytes) byteSize)
                                       (eq! limb input_lo)
                                       (eq! nBytes LLARGE)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.2 RLP of a 20 bytes address  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (rlpAddressConstraints input_hi input_lo ct)
  (if-eq-else nSTEP 1
              (begin  ;; 1
                     (eq! LIMB
                          (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                     (eq! nBYTES 1))
              (begin  ;; 2
                     (eq! nSTEP 16)
                     (if-eq DONE 1
                            (begin (eq! [ACC 1] input_hi)
                                   (vanishes! (shift [ACC 1] -4))
                                   (eq! [ACC 2] input_lo)
                                   (did-change! (shift LC -2))
                                   (eq! (shift LIMB -2)
                                        (* (+ RLP_PREFIX_INT_SHORT 20) (^ 256 LLARGEMO)))
                                   (eq! (shift nBYTES -2) 1)
                                   (eq! (prev LIMB)
                                        (* input_hi (^ 256 12)))
                                   (eq! (prev nBYTES) 4)
                                   (eq! LIMB input_lo)
                                   (eq! nBYTES LLARGE))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    3.3 RLP of a 32 bytes STorage Key  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (rlpStorageKeyConstraints input_hi input_lo ct)
  (begin (eq! nSTEP LLARGE)
         (if-eq DONE 1
                (begin (eq! [ACC 1] input_hi)
                       (eq! [ACC 2] input_lo)
                       (did-change! (shift LC -2))
                       (eq! (shift LIMB -2)
                            (* (+ RLP_PREFIX_INT_SHORT 32) (^ 256 LLARGEMO)))
                       (eq! (shift nBYTES -2) 1)
                       (eq! (prev LIMB) input_hi)
                       (eq! (prev nBYTES) LLARGE)
                       (eq! LIMB input_lo)
                       (eq! nBYTES LLARGE)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4 Phase Heartbeat  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.1 Phase 0 : RLP prefix  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseRlpPrefix-bytetypeprefix (:guard IS_PHASE_RLP_PREFIX);; 4.1.1
  (if-zero (prev IS_PHASE_RLP_PREFIX)
           (begin (eq! nSTEP 1)
                  (vanishes! (+ (- 1 LT)         ;;1.b
                                (- 1 LX)         ;;1.c
                                PHASE_END        ;;1.d
                                (- 1 (next LT))  ;;1.g
                                (next LX)))      ;;1.h
                  (if-zero TYPE
                           (eq! LC_CORRECTION 1) ;; 1.e
                           (begin                ;;1.f
                                  (vanishes! LC_CORRECTION)
                                  (eq! LIMB
                                       (* TYPE (^ 256 LLARGEMO)))
                                  (eq! nBYTES 1)))
                  (eq! DATA_LO TYPE))))

(defconstraint phaseRlpPrefix-rlplt (:guard IS_PHASE_RLP_PREFIX)
  (if-zero (+ (- 1 LT) LX)
           (begin (vanishes! (+ LC_CORRECTION PHASE_END))
                  (eq! [INPUT 1] RLP_LT_BYTESIZE)
                  (eq! nSTEP 8)
                  (rlpPrefixOfByteString [INPUT 1]
                                         CT
                                         nSTEP
                                         DONE
                                         IS_PHASE_RLP_PREFIX
                                         ACC_BYTESIZE
                                         POWER
                                         BIT
                                         [ACC 1]
                                         [ACC 2]
                                         LC
                                         LIMB
                                         nBYTES)
                  (if-eq DONE 1
                         (vanishes! (+ (next LT)
                                       (- 1 (next LX))))))))

(defconstraint phaseRlpPrefix-rlplx (:guard IS_PHASE_RLP_PREFIX)
  (if-zero (+ LT (- 1 LX))
           (begin (vanishes! LC_CORRECTION)
                  (eq! [INPUT 1] RLP_LX_BYTESIZE)
                  (eq! nSTEP 8)
                  (rlpPrefixOfByteString [INPUT 1]
                                         CT
                                         nSTEP
                                         DONE
                                         IS_PHASE_RLP_PREFIX
                                         ACC_BYTESIZE
                                         POWER
                                         BIT
                                         [ACC 1]
                                         [ACC 2]
                                         LC
                                         LIMB
                                         nBYTES)
                  (if-eq DONE 1 (eq! PHASE_END 1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                     ;;
;;    4.2 Phase 1, 2, 3, 4, 5 , 6 , 8 : RLP(integer))  ;;
;;                                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (flag-sum-integer-phase)
  (force-bin (+ IS_PHASE_CHAIN_ID
                IS_PHASE_NONCE
                IS_PHASE_GAS_PRICE
                IS_PHASE_MAX_PRIORITY_FEE_PER_GAS
                IS_PHASE_MAX_FEE_PER_GAS
                IS_PHASE_GAS_LIMIT
                IS_PHASE_VALUE)))

(defun (flag-sum-integer-phase-in-8-rows)
  (force-bin (- (flag-sum-integer-phase) IS_PHASE_VALUE)))

(defconstraint phaseInteger (:guard (flag-sum-integer-phase))
  (begin (if-zero [INPUT 1]
                  (begin (eq! nSTEP 1)
                         (eq! LIMB
                              (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                         (eq! nBYTES 1))
                  (begin (eq! nSTEP
                              (+ (* 8 (flag-sum-integer-phase-in-8-rows)) (* LLARGE IS_PHASE_VALUE)))
                         (rlpPrefixInt [INPUT 1] CT nSTEP DONE [BYTE 1] [ACC 1] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)
                         (if-eq DONE 1 (limbShifting [INPUT 1] POWER ACC_BYTESIZE LIMB nBYTES))))
         (if-eq DONE 1
                (begin (eq! PHASE_END 1)
                       (if-eq (+ IS_PHASE_NONCE
                                 IS_PHASE_GAS_PRICE
                                 IS_PHASE_MAX_FEE_PER_GAS
                                 IS_PHASE_GAS_LIMIT
                                 IS_PHASE_VALUE) 1
                              (eq! DATA_LO [INPUT 1]))
                       (if-eq IS_PHASE_MAX_PRIORITY_FEE_PER_GAS 1
                              (eq! (next DATA_HI) [INPUT 1]))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.3 Phase 7 : Address    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseTo (:guard IS_PHASE_TO)
  (begin (rlpAddressConstraints [INPUT 1] [INPUT 2] CT)
         (if-eq DONE 1
                (begin (eq! PHASE_END 1)
                       (eq! DATA_HI [INPUT 1])
                       (eq! DATA_LO [INPUT 2])
                       (if-eq-else nSTEP 1
                                   (eq! (next DATA_HI) 1)
                                   (vanishes! (next DATA_HI)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.4 Phase 9 : Data  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseData-indexdata-update (:guard IS_PHASE_DATA)
  (if-eq-else IS_PREFIX 1
              (vanishes! INDEX_DATA)
              (if-zero (+ (prev IS_PREFIX)
                          (* (- 1 (prev LC))
                             (- 1 (prev LC_CORRECTION))))
                       (did-inc! INDEX_DATA 1)
                       (remained-constant! INDEX_DATA))))

(defconstraint phaseData-nolccorrection-noend (:guard IS_PHASE_DATA)
  (if-zero (* LC_CORRECTION (- 1 IS_PREFIX))
           (vanishes! PHASE_END)))

(defconstraint phaseData-endphase (:guard IS_PHASE_DATA)
  (if-zero (+ IS_PREFIX (- 1 LC_CORRECTION) (- 1 DONE))
           (eq! PHASE_END 1)))

(defconstraint phaseData-firstrow-initialisation (:guard IS_PHASE_DATA)
  (if-zero (prev IS_PHASE_DATA)
           (begin (eq! IS_PREFIX 1)
                  (if-zero PHASE_SIZE
                           (eq! nSTEP 1)
                           (eq! nSTEP 8))
                  (eq! DATA_HI DATA_GAS_COST)
                  (eq! DATA_LO PHASE_SIZE))))

(defconstraint phaseData-trivialcase (:guard IS_PHASE_DATA)
  (if-not-zero (* IS_PREFIX (- 8 nSTEP))
               (begin (eq! LIMB
                           (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                      (eq! nBYTES 1)
                      (vanishes! (+ LC_CORRECTION
                                    (next IS_PREFIX)
                                    (- 1 (next LC_CORRECTION))))
                      (eq! (next nSTEP) 1))))

(defconstraint phaseData-rlpprefix (:guard IS_PHASE_DATA)
  (if-not-zero (* IS_PREFIX (- nSTEP 1))
               (begin (will-remain-constant! PHASE_SIZE)
                      (will-remain-constant! DATA_GAS_COST)
                      (if-eq-else PHASE_SIZE 1
                                  (begin (rlpPrefixInt [INPUT 1]
                                                       CT
                                                       nSTEP
                                                       DONE
                                                       [BYTE 1]
                                                       [ACC 1]
                                                       ACC_BYTESIZE
                                                       POWER
                                                       BIT
                                                       BIT_ACC
                                                       LIMB
                                                       LC
                                                       nBYTES)
                                         (if-not-eq COUNTER (- nSTEP 2) (vanishes! LC))
                                         (if-eq DONE 1
                                                (begin (eq! (+ (prev LC_CORRECTION) LC_CORRECTION)
                                                            1)
                                                       (eq! (next [INPUT 1])
                                                            (* [INPUT 1] (^ 256 LLARGEMO))))))
                                  (begin (eq! [INPUT 1] PHASE_SIZE)
                                         (vanishes! LC_CORRECTION)
                                         (rlpPrefixOfByteString [INPUT 1]
                                                                CT
                                                                nSTEP
                                                                DONE
                                                                IS_PHASE_RLP_PREFIX
                                                                ACC_BYTESIZE
                                                                POWER
                                                                BIT
                                                                [ACC 1]
                                                                [ACC 2]
                                                                LC
                                                                LIMB
                                                                nBYTES)
                                         (if-not-zero (* (- 1 DONE)
                                                         (- nSTEP (+ COUNTER 2)))
                                                      (vanishes! LIMB_CONSTRUCTED))))
                      (if-eq DONE 1
                             (vanishes! (+ (next IS_PREFIX) (next LC_CORRECTION)))))))

(defconstraint phaseData-dataconcatenation (:guard IS_PHASE_DATA)
  (if-zero (+ IS_PREFIX LC_CORRECTION)
           (begin (eq! nSTEP LLARGE)
                  (if-not-zero PHASE_SIZE
                               (begin (will-dec! PHASE_SIZE 1)
                                      (if-zero [BYTE 1]
                                               (will-dec! DATA_GAS_COST GAS_CONST_G_TX_DATA_ZERO)
                                               (will-dec! DATA_GAS_COST GAS_CONST_G_TX_DATA_NONZERO)))
                               (begin (will-remain-constant! PHASE_SIZE)
                                      (will-remain-constant! DATA_GAS_COST)))
                  (if-zero CT
                           (eq! ACC_BYTESIZE 1)
                           (if-not-zero PHASE_SIZE
                                        (did-inc! ACC_BYTESIZE 1)
                                        (begin (remained-constant! ACC_BYTESIZE)
                                               (vanishes! [BYTE 1]))))
                  (if-eq DONE 1
                         (begin (vanishes! LC_CORRECTION)
                                (vanishes! (prev LC))
                                (eq! [ACC 1] [INPUT 1])
                                (eq! LIMB [INPUT 1])
                                (eq! nBYTES ACC_BYTESIZE)
                                (if-eq-else (^ PHASE_SIZE 2) PHASE_SIZE
                                            (begin (eq! (next nSTEP) 2)
                                                   (vanishes! (- 1 (next LC_CORRECTION)))
                                                   (eq! (next PHASE_SIZE) (shift PHASE_SIZE 2))
                                                   (eq! (next DATA_GAS_COST) (shift DATA_GAS_COST 2)))
                                            (vanishes! (next LC_CORRECTION))))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    4.5 Phase 10 : AccessList  ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseAccessList-stillphase-noend (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero PHASE_SIZE
               (vanishes! PHASE_END)))

(defconstraint phaseAccessList-endphase (:guard IS_PHASE_ACCESS_LIST)
  (if-zero (+ PHASE_SIZE (- 1 DONE))
           (eq! PHASE_END 1)))

;; 4.5.2.3
(defconstraint phaseAccessList-firstrow (:guard IS_PHASE_ACCESS_LIST)
  (if-zero (prev IS_PHASE_ACCESS_LIST)
           (begin (eq! DATA_HI nKEYS)
                  (eq! DATA_LO nADDR)
                  (vanishes! (+ (- 1 IS_PREFIX) [DEPTH 1] [DEPTH 2]))
                  (eq! [INPUT 1] PHASE_SIZE)
                  (if-zero nADDR
                           (begin (eq! nSTEP 1)
                                  (eq! LIMB
                                       (* RLP_PREFIX_LIST_SHORT (^ 256 LLARGEMO)))
                                  (eq! nBYTES 1))
                           (eq! nSTEP 8)))))

(defconstraint phaseAccessList-rlpprefix (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* (- 1 [DEPTH 1]) nADDR)
               (begin (rlpPrefixOfByteString [INPUT 1]
                                             CT
                                             nSTEP
                                             DONE
                                             IS_PHASE_ACCESS_LIST
                                             ACC_BYTESIZE
                                             POWER
                                             BIT
                                             [ACC 1]
                                             [ACC 2]
                                             LC
                                             LIMB
                                             nBYTES)
                      (if-eq DONE 1
                             (begin (eq! (next IS_PREFIX) 1)
                                    (eq! (next [DEPTH 1]) 1)
                                    (vanishes! (next [DEPTH 2])))))))

(defconstraint phaseAccessList-rlpprefix-tupleitem (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* IS_PREFIX [DEPTH 1] (- 1 [DEPTH 2]))
               (begin (eq! [INPUT 1] ACCESS_TUPLE_BYTESIZE)
                      (eq! nSTEP 8)
                      (rlpPrefixOfByteString [INPUT 1]
                                             CT
                                             nSTEP
                                             DONE
                                             IS_PHASE_ACCESS_LIST
                                             ACC_BYTESIZE
                                             POWER
                                             BIT
                                             [ACC 1]
                                             [ACC 2]
                                             LC
                                             LIMB
                                             nBYTES)
                      (if-eq DONE 1
                             (begin (vanishes! (next IS_PREFIX))
                                    (eq! (next [DEPTH 1]) 1)
                                    (vanishes! (next [DEPTH 2])))))))

(defconstraint phaseAccessList-rlpAddr (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* (- 1 IS_PREFIX) [DEPTH 1] (- 1 [DEPTH 2]))
               (begin (eq! [INPUT 1] ADDR_HI)
                      (eq! [INPUT 2] ADDR_LO)
                      (eq! nSTEP 16)
                      (rlpAddressConstraints [INPUT 1] [INPUT 2] CT)
                      (if-eq DONE 1
                             (eq! 1
                                  (* (next IS_PREFIX) (next [DEPTH 1]) (next [DEPTH 2])))))))

(defconstraint phaseAccessList-rlpprefix-listStoKeys (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* IS_PREFIX [DEPTH 1] [DEPTH 2])
               (if-zero nKEYS_PER_ADDR
                        (begin (eq! nSTEP 1)
                               (eq! LIMB
                                    (* RLP_PREFIX_LIST_SHORT (^ 256 LLARGEMO)))
                               (eq! nBYTES 1))
                        (begin (eq! nSTEP 8)
                               (eq! [INPUT 1] (* 33 nKEYS_PER_ADDR))
                               (rlpPrefixOfByteString [INPUT 1]
                                                      CT
                                                      nSTEP
                                                      DONE
                                                      IS_PHASE_ACCESS_LIST
                                                      ACC_BYTESIZE
                                                      POWER
                                                      BIT
                                                      [ACC 1]
                                                      [ACC 2]
                                                      LC
                                                      LIMB
                                                      nBYTES)))))

(defconstraint phaseAccessList-rlp-StoKeys (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* (- 1 IS_PREFIX) [DEPTH 1] [DEPTH 2])
               (rlpStorageKeyConstraints [INPUT 1] [INPUT 2] CT)))

(defconstraint phaseAccessList-depth2loopintrication (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* [DEPTH 2] DONE)
               (if-not-zero nKEYS_PER_ADDR
                            (vanishes! (+ (next IS_PREFIX)
                                          (- 1 (next [DEPTH 1]))
                                          (- 1 (next [DEPTH 2]))))
                            (begin (vanishes! ACCESS_TUPLE_BYTESIZE)
                                   (if-not-zero nADDR
                                                (vanishes! (+ (- 1 (next IS_PREFIX))
                                                              (- 1 (next [DEPTH 1]))
                                                              (next [DEPTH 2]))))))))

(defconstraint phaseAccessList-sizeupdate (:guard IS_PHASE_ACCESS_LIST)
  (if-zero [DEPTH 1]
           (will-remain-constant! PHASE_SIZE)
           (begin (did-dec! PHASE_SIZE (* LC nBYTES))
                  (if-zero (* IS_PREFIX (- 1 [DEPTH 2]))
                           (did-dec! ACCESS_TUPLE_BYTESIZE (* LC nBYTES)))
                  (if-zero CT
                           (begin (did-dec! nADDR
                                            (* IS_PREFIX (- 1 [DEPTH 2])))
                                  (did-dec! nKEYS
                                            (* (- 1 IS_PREFIX) [DEPTH 2])))))))

;; 4.5.2.14
(defconstraint phaseAccessList-nKeysperAddr-update (:guard IS_PHASE_ACCESS_LIST)
  (if-zero (+ CT
              (* IS_PREFIX (- 1 [DEPTH 2])))
           (did-dec! nKEYS_PER_ADDR
                     (* (- 1 IS_PREFIX) [DEPTH 2]))))

;; 4.5.2.15
(defconstraint phaseAccessList-updateAddrLookUp (:guard IS_PHASE_ACCESS_LIST)
  (if-not-zero (* [DEPTH 1]
                  (- nADDR
                     (- (prev nADDR) 1)))
               (begin (remained-constant! ADDR_HI)
                      (remained-constant! ADDR_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.6 Phase 11 : Beta / w  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseBeta-firstrow (:guard IS_PHASE_BETA)
  (if-zero (prev IS_PHASE_BETA)
           (begin (vanishes! (+ (- 1 LT) LX))
                  (eq! nSTEP 8))))

(defun (w-minus-two-seven)
  (- [INPUT 1] UNPROTECTED_V))

(defun (w-minus-two-beta-minus-protected-base-V)
  (- [INPUT 1]
     (+ (* 2 (next [INPUT 1]))
        PROTECTED_BASE_V)))

(defconstraint phaseBeta-rlp-w (:guard IS_PHASE_BETA)
  (if-not-zero (* LT (- 1 LX))
               (begin (rlpPrefixInt [INPUT 1] CT nSTEP DONE [BYTE 1] [ACC 1] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)
                      (if-eq DONE 1
                             (begin (limbShifting [INPUT 1] POWER ACC_BYTESIZE LIMB nBYTES)
                                    (vanishes! LC_CORRECTION)
                                    (if-eq-else (^ (w-minus-two-seven) 2) (w-minus-two-seven)
                                                (eq! PHASE_END 1)
                                                (begin (vanishes! (+ PHASE_END
                                                                     (next LT)
                                                                     (- 1 (next LX))
                                                                     (- 1 (next IS_PREFIX))))
                                                       (is-binary (w-minus-two-beta-minus-protected-base-V)))))))))

(defconstraint phaseBeta-rlp-beta (:guard IS_PHASE_BETA)
  (if-not-zero (* LX IS_PREFIX)
               (begin (eq! nSTEP 8)
                      (rlpPrefixInt [INPUT 1] CT nSTEP DONE [BYTE 1] [ACC 1] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)
                      (if-eq DONE 1
                             (begin (limbShifting [INPUT 1] POWER ACC_BYTESIZE LIMB nBYTES)
                                    (vanishes! (+ LC_CORRECTION
                                                  PHASE_END
                                                  (next IS_PREFIX)
                                                  (next LT)
                                                  (- 1 (next LX))
                                                  (next LC_CORRECTION)
                                                  (- 1 (next PHASE_END))
                                                  (next LC_CORRECTION)))
                                    (eq! (next nSTEP) 1)
                                    (eq! (next LIMB)
                                         (+ (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO))
                                            (* RLP_PREFIX_INT_SHORT (^ 256 14))))
                                    (eq! (next nBYTES) 2))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    4.7 Phase 12 : y   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseY (:guard IS_PHASE_Y)
  (begin (is-binary [INPUT 1])
         (eq! nSTEP 1)
         (if-zero [INPUT 1]
                  (eq! LIMB
                       (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                  (eq! LIMB
                       (* [INPUT 1] (^ 256 LLARGEMO))))
         (eq! nBYTES 1)
         (eq! PHASE_END 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.8 Phase 13-14 : r & s  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseRandS (:guard (+ IS_PHASE_R IS_PHASE_S))
  (begin (if-zero (+ (~ [INPUT 1]) (~ [INPUT 2]))
                  (begin (eq! nSTEP 1)
                         (eq! LIMB
                              (* RLP_PREFIX_INT_SHORT (^ 256 LLARGEMO)))
                         (eq! nBYTES 1))
                  (begin (eq! nSTEP LLARGE)
                         (rlpPrefixLongInt [INPUT 1] [INPUT 2] CT nSTEP DONE [BYTE 1] [BYTE 2] [ACC 1] [ACC 2] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)))
         (if-eq DONE 1 (eq! PHASE_END 1))))


