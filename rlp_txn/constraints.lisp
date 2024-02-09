(module rlpTxn)

(defpurefun (if-not-eq A B then)
  (if-not-zero (- A B)
               then))

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
(defpurefun (phase-constancy PHASE C)
  (if-eq (* PHASE (prev PHASE)) 1
         (remained-constant! C)))

;; Definition phase-incrementing
(defpurefun (phase-incrementing PHASE C)
  (if-eq (* PHASE (prev PHASE)) 1
         (or! (remained-constant! C) (did-inc! C 1))))

;; Definition phase-decrementing
(defpurefun (phase-decrementing PHASE C)
  (if-eq (* PHASE (prev PHASE)) 1
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
  (if-zero (* [PHASE PHASE_DATA_VALUE] IS_PREFIX)
           (counter-incrementing CT LIMB_CONSTRUCTED)))

(defconstraint phaseRlpPrefix-constancy ()
  (begin (phase-constancy [PHASE PHASE_RLP_PREFIX_VALUE] RLP_LT_BYTESIZE)
         (phase-constancy [PHASE PHASE_RLP_PREFIX_VALUE] RLP_LX_BYTESIZE)
         (phase-constancy [PHASE PHASE_RLP_PREFIX_VALUE] DATA_HI)
         (phase-constancy [PHASE PHASE_RLP_PREFIX_VALUE] DATA_LO)))

(defconstraint phaseData-decrementing ()
  (phase-decrementing [PHASE PHASE_DATA_VALUE] IS_PREFIX))

(defconstraint phasek-constancies ()
  (for i
       [3:11]
       (begin (phase-constancy [PHASE i] DATA_HI)
              (phase-constancy [PHASE i] DATA_LO))))

(defconstraint block-constancies ()
  (block-constant ABS_TX_NUM ABS_TX_NUM_INFINY))

;; 2.3.1.7 (debug 2.3.1.11)
;; (defconstraint phase9-incrementing ()
;;   (begin
;;    (debug (phase-incrementing [PHASE 9] INDEX_DATA))))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    2.3.2 Global Phase Constraints    ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phase-id-to-phase-flag ()
  (eq! PHASE_ID
       (reduce +
               (for k [1 : 15] (* k [PHASE k])))))

;; 2.3.2.1
(defconstraint initial-stamp (:domain {0})
  (vanishes! ABS_TX_NUM))

;; 2.3.2.2
(defconstraint ABS_TX_NUM-is-zero ()
  (if-zero ABS_TX_NUM
           (vanishes! (reduce + (for i [1 : 15] [PHASE i])))
           (eq! 1
                (reduce + (for i [1 : 15] [PHASE i])))))

;; 2.3.2.4
(defconstraint ABS_TX_NUM-evolution ()
  (eq! ABS_TX_NUM
       (+ (prev ABS_TX_NUM)
          (* [PHASE PHASE_RLP_PREFIX_VALUE] (remained-constant! [PHASE PHASE_RLP_PREFIX_VALUE])))))

;; 2.3.2.6
(defconstraint LT-and-LX ()
  (if-eq (reduce + (for i [2 : 11] [PHASE i])) 1
         (begin (eq! LT 1)
                (eq! LX 1))))

;; 2.3.2.7
(defconstraint LT-only ()
  (if-eq (reduce + (for i [13 : 15] [PHASE i])) 1
         (begin (eq! 1 LT)
                (vanishes! LX))))

;; 2.3.2.8
(defconstraint no-done-no-end ()
  (if-zero DONE
           (vanishes! PHASE_END)))

;; 2.3.2.9
(defconstraint no-end-no-changephase (:guard ABS_TX_NUM)
  (if-zero PHASE_END
           (vanishes! (reduce +
                              (for i
                                   [1 : 15]
                                   (* i
                                      (- (next [PHASE i]) [PHASE i])))))))

;; 2.3.2.10
(defconstraint phase-transition ()
  (if-eq PHASE_END 1
         (begin (if-eq [PHASE PHASE_RLP_PREFIX_VALUE] 1
                       (if-zero TYPE
                                (eq! (next [PHASE PHASE_NONCE_VALUE]) 1)
                                (eq! (next [PHASE PHASE_CHAIN_ID_VALUE]) 1)))
                (if-eq [PHASE PHASE_CHAIN_ID_VALUE] 1
                       (eq! (next [PHASE PHASE_NONCE_VALUE]) 1))
                (if-eq [PHASE PHASE_NONCE_VALUE] 1
                       (if-eq-else TYPE 2
                                   (eq! (next [PHASE PHASE_MAX_PRIORITY_FEE_PER_GAS_VALUE]) 1)
                                   (eq! (next [PHASE PHASE_GAS_PRICE_VALUE]) 1)))
                (if-eq [PHASE PHASE_GAS_PRICE_VALUE] 1
                       (eq! (next [PHASE PHASE_GAS_LIMIT_VALUE]) 1))
                (if-eq [PHASE PHASE_MAX_PRIORITY_FEE_PER_GAS_VALUE] 1
                       (eq! (next [PHASE PHASE_MAX_FEE_PER_GAS_VALUE]) 1))
                (if-eq [PHASE PHASE_MAX_FEE_PER_GAS_VALUE] 1
                       (eq! (next [PHASE PHASE_GAS_LIMIT_VALUE]) 1))
                (if-eq [PHASE PHASE_GAS_LIMIT_VALUE] 1
                       (eq! (next [PHASE PHASE_TO_VALUE]) 1))
                (if-eq [PHASE PHASE_TO_VALUE] 1
                       (eq! (next [PHASE PHASE_VALUE_VALUE]) 1))
                (if-eq [PHASE PHASE_VALUE_VALUE] 1
                       (eq! (next [PHASE PHASE_DATA_VALUE]) 1))
                (if-eq [PHASE PHASE_DATA_VALUE] 1
                       (begin (debug (vanishes! PHASE_SIZE))
                              (vanishes! DATAGASCOST)
                              (if-zero TYPE
                                       (eq! (next [PHASE PHASE_BETA_VALUE]) 1)
                                       (eq! (next [PHASE PHASE_ACCESS_LIST_VALUE]) 1))))
                (if-eq [PHASE PHASE_ACCESS_LIST_VALUE] 1
                       (begin (debug (vanishes! PHASE_SIZE))
                              (vanishes! nADDR)
                              (vanishes! nKEYS)
                              (vanishes! nKEYS_PER_ADDR)
                              (eq! (next [PHASE PHASE_Y_VALUE]) 1)))
                (if-eq [PHASE PHASE_BETA_VALUE] 1
                       (eq! (next [PHASE PHASE_R_VALUE]) 1))
                (if-eq [PHASE PHASE_Y_VALUE] 1
                       (eq! (next [PHASE PHASE_R_VALUE]) 1))
                (if-eq [PHASE PHASE_R_VALUE] 1
                       (eq! (next [PHASE PHASE_S_VALUE]) 1))
                (if-eq [PHASE PHASE_S_VALUE] 1
                       (begin (vanishes! RLP_LT_BYTESIZE)
                              (vanishes! RLP_LX_BYTESIZE)
                              (eq! (next [PHASE PHASE_RLP_PREFIX_VALUE]) 1))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.3.3 Byte decomposition's loop heartbeat  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; 2.3.3.2 & 2.3.3.3
(defconstraint cy-imply-done (:guard ABS_TX_NUM)
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
  (if-zero (remained-constant! ABS_TX_NUM)
           (begin (eq! INDEX_LT
                       (+ (prev INDEX_LT)
                          (* (prev LC) (prev LT))))
                  (eq! INDEX_LX
                       (+ (prev INDEX_LX)
                          (* (prev LC) (prev LX)))))
           (begin (vanishes! INDEX_LT)
                  (vanishes! INDEX_LX))))

;; 2.3.5.3
(defconstraint rlpbytesize-decreasing ()
  (if-eq 1 (reduce + (for i [2 : 15] [PHASE i]))
         (begin (eq! RLP_LT_BYTESIZE
                     (- (prev RLP_LT_BYTESIZE) (* LC LT nBYTES)))
                (eq! RLP_LX_BYTESIZE
                     (- (prev RLP_LX_BYTESIZE) (* LC LX nBYTES))))))

(defconstraint lc-correction-nullity ()
  (if-zero (+ [PHASE PHASE_RLP_PREFIX_VALUE] [PHASE PHASE_DATA_VALUE] [PHASE PHASE_BETA_VALUE])
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
                      (eq! 1 [PHASE PHASE_S_VALUE]))))

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
                                                            (* (+ INT_SHORT byteSize) (^ 256 LLARGEMO)))
                                                       (eq! (prev nBytes) 1)
                                                       (eq! limb (* input_lo power))
                                                       (eq! nBytes byteSize))))
                                (begin (eq! (+ (shift lc -3) (shift lc -2))
                                            1)
                                       (eq! (shift limb -2)
                                            (* (+ INT_SHORT LLARGE byteSize) (^ 256 LLARGEMO)))
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
                          (* INT_SHORT (^ 256 LLARGEMO)))
                     (eq! nBYTES 1))
              (begin  ;; 2
                     (eq! nSTEP 16)
                     (if-eq DONE 1
                            (begin (eq! [ACC 1] input_hi)
                                   (vanishes! (shift [ACC 1] -4))
                                   (eq! [ACC 2] input_lo)
                                   (did-change! (shift LC -2))
                                   (eq! (shift LIMB -2)
                                        (* (+ INT_SHORT 20) (^ 256 LLARGEMO)))
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
                            (* (+ INT_SHORT 32) (^ 256 LLARGEMO)))
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
(defconstraint phaseRlpPrefix-bytetypeprefix (:guard [PHASE PHASE_RLP_PREFIX_VALUE]);; 4.1.1
  (if-zero (prev [PHASE PHASE_RLP_PREFIX_VALUE])
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

(defconstraint phaseRlpPrefix-rlplt (:guard [PHASE PHASE_RLP_PREFIX_VALUE])
  (if-zero (+ (- 1 LT) LX)
           (begin (vanishes! (+ LC_CORRECTION PHASE_END))
                  (eq! [INPUT 1] RLP_LT_BYTESIZE)
                  (eq! nSTEP 8)
                  (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE PHASE_RLP_PREFIX_VALUE] ACC_BYTESIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                  (if-eq DONE 1
                         (vanishes! (+ (next LT)
                                       (- 1 (next LX))))))))

(defconstraint phaseRlpPrefix-rlplx (:guard [PHASE PHASE_RLP_PREFIX_VALUE])
  (if-zero (+ LT (- 1 LX))
           (begin (vanishes! LC_CORRECTION)
                  (eq! [INPUT 1] RLP_LX_BYTESIZE)
                  (eq! nSTEP 8)
                  (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE PHASE_RLP_PREFIX_VALUE] ACC_BYTESIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                  (if-eq DONE 1 (eq! PHASE_END 1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                     ;;
;;    4.2 Phase 1, 2, 3, 4, 5 , 6 , 8 : RLP(integer))  ;;
;;                                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseInteger (:guard (+ (reduce + (for i [2 : 7] [PHASE i]))
      [PHASE PHASE_VALUE_VALUE]))
  (begin (if-zero [INPUT 1]
                  (begin (eq! nSTEP 1)
                         (eq! LIMB
                              (* INT_SHORT (^ 256 LLARGEMO)))
                         (eq! nBYTES 1))
                  (begin (eq! nSTEP
                              (+ (* 8
                                    (reduce + (for i [2 : 7] [PHASE i])))
                                 (* LLARGE [PHASE PHASE_VALUE_VALUE])))
                         (rlpPrefixInt [INPUT 1] CT nSTEP DONE [BYTE 1] [ACC 1] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)
                         (if-eq DONE 1 (limbShifting [INPUT 1] POWER ACC_BYTESIZE LIMB nBYTES))))
         (if-eq DONE 1
                (begin (eq! PHASE_END 1)
                       (if-eq (+ [PHASE PHASE_NONCE_VALUE] [PHASE PHASE_GAS_PRICE_VALUE] [PHASE PHASE_MAX_FEE_PER_GAS_VALUE] [PHASE PHASE_GAS_LIMIT_VALUE] [PHASE PHASE_VALUE_VALUE]) 1 (eq! DATA_LO [INPUT 1]))
                       (if-eq [PHASE PHASE_MAX_PRIORITY_FEE_PER_GAS_VALUE] 1
                              (eq! (next DATA_HI) [INPUT 1]))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.3 Phase 7 : Address    ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseTo (:guard [PHASE PHASE_TO_VALUE])
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
(defconstraint phaseData-indexdata-update (:guard [PHASE PHASE_DATA_VALUE])
  (if-eq-else IS_PREFIX 1
              (vanishes! INDEX_DATA)
              (if-zero (+ (prev IS_PREFIX)
                          (* (- 1 (prev LC))
                             (- 1 (prev LC_CORRECTION))))
                       (did-inc! INDEX_DATA 1)
                       (remained-constant! INDEX_DATA))))

(defconstraint phaseData-nolccorrection-noend (:guard [PHASE PHASE_DATA_VALUE])
  (if-zero (* LC_CORRECTION (- 1 IS_PREFIX))
           (vanishes! PHASE_END)))

(defconstraint phaseData-endphase (:guard [PHASE PHASE_DATA_VALUE])
  (if-zero (+ IS_PREFIX (- 1 LC_CORRECTION) (- 1 DONE))
           (eq! PHASE_END 1)))

(defconstraint phaseData-firstrow-initialisation (:guard [PHASE PHASE_DATA_VALUE])
  (if-zero (prev [PHASE PHASE_DATA_VALUE])
           (begin (eq! IS_PREFIX 1)
                  (if-zero PHASE_SIZE
                           (eq! nSTEP 1)
                           (eq! nSTEP 8))
                  (eq! DATA_HI DATAGASCOST)
                  (eq! DATA_LO PHASE_SIZE))))

(defconstraint phaseData-trivialcase (:guard [PHASE PHASE_DATA_VALUE])
  (if-not-zero (* IS_PREFIX (- 8 nSTEP))
               (begin (eq! LIMB
                           (* INT_SHORT (^ 256 LLARGEMO)))
                      (eq! nBYTES 1)
                      (vanishes! (+ LC_CORRECTION
                                    (next IS_PREFIX)
                                    (- 1 (next LC_CORRECTION))))
                      (eq! (next nSTEP) 1))))

(defconstraint phaseData-rlpprefix (:guard [PHASE PHASE_DATA_VALUE])
  (if-not-zero (* IS_PREFIX (- nSTEP 1))
               (begin (will-remain-constant! PHASE_SIZE)
                      (will-remain-constant! DATAGASCOST)
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
                                                                [PHASE PHASE_RLP_PREFIX_VALUE]
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

(defconstraint phaseData-dataconcatenation (:guard [PHASE PHASE_DATA_VALUE])
  (if-zero (+ IS_PREFIX LC_CORRECTION)
           (begin (eq! nSTEP LLARGE)
                  (if-not-zero PHASE_SIZE
                               (begin (will-dec! PHASE_SIZE 1)
                                      (if-zero [BYTE 1]
                                               (will-dec! DATAGASCOST G_TXDATA_ZERO)
                                               (will-dec! DATAGASCOST G_TXDATA_NONZERO)))
                               (begin (will-remain-constant! PHASE_SIZE)
                                      (will-remain-constant! DATAGASCOST)))
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
                                                   (eq! (next DATAGASCOST) (shift DATAGASCOST 2)))
                                            (vanishes! (next LC_CORRECTION))))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.5 Phase 10 : AccessList  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseAccessList-stillphase-noend (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-not-zero PHASE_SIZE
               (vanishes! PHASE_END)))

(defconstraint phaseAccessList-endphase (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-zero (+ PHASE_SIZE (- 1 DONE))
           (eq! PHASE_END 1)))

;; 4.5.2.3
(defconstraint phaseAccessList-firstrow (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-zero (prev [PHASE PHASE_ACCESS_LIST_VALUE])
           (begin (eq! DATA_HI nKEYS)
                  (eq! DATA_LO nADDR)
                  (vanishes! (+ (- 1 IS_PREFIX) [DEPTH 1] [DEPTH 2]))
                  (eq! [INPUT 1] PHASE_SIZE)
                  (if-zero nADDR
                           (begin (eq! nSTEP 1)
                                  (eq! LIMB
                                       (* LIST_SHORT (^ 256 LLARGEMO)))
                                  (eq! nBYTES 1))
                           (eq! nSTEP 8)))))

(defconstraint phaseAccessList-rlpprefix (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-not-zero (* (- 1 [DEPTH 1]) nADDR)
               (begin (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE PHASE_ACCESS_LIST_VALUE] ACC_BYTESIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                      (if-eq DONE 1
                             (begin (eq! (next IS_PREFIX) 1)
                                    (eq! (next [DEPTH 1]) 1)
                                    (vanishes! (next [DEPTH 2])))))))

(defconstraint phaseAccessList-rlpprefix-tupleitem (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-not-zero (* IS_PREFIX [DEPTH 1] (- 1 [DEPTH 2]))
               (begin (eq! [INPUT 1] ACCESS_TUPLE_BYTESIZE)
                      (eq! nSTEP 8)
                      (rlpPrefixOfByteString [INPUT 1] CT nSTEP DONE [PHASE PHASE_ACCESS_LIST_VALUE] ACC_BYTESIZE POWER BIT [ACC 1] [ACC 2] LC LIMB nBYTES)
                      (if-eq DONE 1
                             (begin (vanishes! (next IS_PREFIX))
                                    (eq! (next [DEPTH 1]) 1)
                                    (vanishes! (next [DEPTH 2])))))))

(defconstraint phaseAccessList-rlpAddr (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-not-zero (* (- 1 IS_PREFIX) [DEPTH 1] (- 1 [DEPTH 2]))
               (begin (eq! [INPUT 1] ADDR_HI)
                      (eq! [INPUT 2] ADDR_LO)
                      (eq! nSTEP 16)
                      (rlpAddressConstraints [INPUT 1] [INPUT 2] CT)
                      (if-eq DONE 1
                             (eq! 1
                                  (* (next IS_PREFIX) (next [DEPTH 1]) (next [DEPTH 2])))))))

(defconstraint phaseAccessList-rlpprefix-listStoKeys (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-not-zero (* IS_PREFIX [DEPTH 1] [DEPTH 2])
               (if-zero nKEYS_PER_ADDR
                        (begin (eq! nSTEP 1)
                               (eq! LIMB
                                    (* LIST_SHORT (^ 256 LLARGEMO)))
                               (eq! nBYTES 1))
                        (begin (eq! nSTEP 8)
                               (eq! [INPUT 1] (* 33 nKEYS_PER_ADDR))
                               (rlpPrefixOfByteString [INPUT 1]
                                                      CT
                                                      nSTEP
                                                      DONE
                                                      [PHASE PHASE_ACCESS_LIST_VALUE]
                                                      ACC_BYTESIZE
                                                      POWER
                                                      BIT
                                                      [ACC 1]
                                                      [ACC 2]
                                                      LC
                                                      LIMB
                                                      nBYTES)))))

(defconstraint phaseAccessList-rlp-StoKeys (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-not-zero (* (- 1 IS_PREFIX) [DEPTH 1] [DEPTH 2])
               (rlpStorageKeyConstraints [INPUT 1] [INPUT 2] CT)))

(defconstraint phaseAccessList-depth2loopintrication (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
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

(defconstraint phaseAccessList-sizeupdate (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
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
(defconstraint phaseAccessList-nKeysperAddr-update (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-zero (+ CT
              (* IS_PREFIX (- 1 [DEPTH 2])))
           (did-dec! nKEYS_PER_ADDR
                     (* (- 1 IS_PREFIX) [DEPTH 2]))))

;; 4.5.2.15
(defconstraint phaseAccessList-updateAddrLookUp (:guard [PHASE PHASE_ACCESS_LIST_VALUE])
  (if-zero (+ [DEPTH 2]
              (- (prev nADDR) nADDR))
           (begin (remained-constant! ADDR_HI)
                  (remained-constant! ADDR_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.6 Phase 11 : Beta / w  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseBeta-firstrow (:guard [PHASE PHASE_BETA_VALUE])
  (if-zero (prev [PHASE PHASE_BETA_VALUE])
           (begin (vanishes! (+ (- 1 LT) LX))
                  (eq! nSTEP 8))))

(defun (w-minus-two-seven)
  (- [INPUT 1] 27))

(defconstraint phaseBeta-rlp-w (:guard [PHASE PHASE_BETA_VALUE])
  (if-not-zero (* LT (- 1 LX))
               (begin (rlpPrefixInt [INPUT 1] CT nSTEP DONE [BYTE 1] [ACC 1] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)
                      (if-eq DONE 1
                             (begin (limbShifting [INPUT 1] POWER ACC_BYTESIZE LIMB nBYTES)
                                    (vanishes! LC_CORRECTION)
                                    (if-eq-else (^ (w-minus-two-seven) 2) (w-minus-two-seven)
                                                (eq! PHASE_END 1)
                                                (begin (vanishes! (+ (next PHASE_END)
                                                                     (next LT)
                                                                     (- 1 (next LX))
                                                                     (- 1 (next IS_PREFIX)))))))))))

(defconstraint phaseBeta-rlp-beta (:guard [PHASE PHASE_BETA_VALUE])
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
                                         (+ (* INT_SHORT (^ 256 LLARGEMO))
                                            (* INT_SHORT (^ 256 14))))
                                    (eq! (next nBYTES) 2))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    4.7 Phase 12 : y   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseY (:guard [PHASE PHASE_Y_VALUE])
  (begin (is-binary [INPUT 1])
         (eq! nSTEP 1)
         (if-zero [INPUT 1]
                  (eq! LIMB
                       (* INT_SHORT (^ 256 LLARGEMO)))
                  (eq! LIMB
                       (* [INPUT 1] (^ 256 LLARGEMO))))
         (eq! nBYTES 1)
         (eq! PHASE_END 1)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    4.8 Phase 13-14 : r & s  ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint phaseRandS (:guard (+ [PHASE PHASE_R_VALUE] [PHASE PHASE_S_VALUE]))
  (begin (if-zero (+ (~ [INPUT 1]) (~ [INPUT 2]))
                  (begin (eq! nSTEP 1)
                         (eq! LIMB
                              (* INT_SHORT (^ 256 LLARGEMO)))
                         (eq! nBYTES 1))
                  (begin (eq! nSTEP 16)
                         (rlpPrefixLongInt [INPUT 1] [INPUT 2] CT nSTEP DONE [BYTE 1] [BYTE 1] [ACC 1] [ACC 2] ACC_BYTESIZE POWER BIT BIT_ACC LIMB LC nBYTES)))
         (if-eq DONE 1 (eq! PHASE_END 1))))


