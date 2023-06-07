(module rom)

(defconst
  ;; 0xC5D2460186F7233C927E7DB2DCC703C0
  EMPTY_CODE_HASH_HI 262949717399590921288928019264691438528
  ;; 0xE500B653CA82273B7BFAD8045D85A470
  EMPTY_CODE_HASH_LO  304396909071904405792975023732328604784)

(defconstraint initialization (:domain {0}) (vanishes! CODE_FRAGMENT_INDEX))
(defconstraint codesize-reached-origin (:domain {-1}) (eq! CODESIZE_REACHED 1))
(defconstraint padding-bit-reached (:domain {-1}) (vanishes! PADDING_BIT))
(defconstraint cycle-finishes (:domain {-1}) (if-not-zero CODE_FRAGMENT_INDEX
                                                          (begin (eq! COUNTER 15)
                                                                 (eq! CYCLIC_BIT 1)
                                                                 (vanishes! PADDED_BYTECODE_BYTE))))

(defun (in-padding) (is-zero CODE_FRAGMENT_INDEX))
(defun (not-in-padding) CODE_FRAGMENT_INDEX)

(defconstraint padding (:guard (in-padding))
  (begin
   (vanishes! IS_INITCODE)
   (vanishes! SC_ADDRESS_HI)
   (vanishes! SC_ADDRESS_LO)
   (vanishes! ADDRESS_INDEX)
   (vanishes! COUNTER)
   (vanishes! CYCLIC_BIT)
   (vanishes! CODESIZE)
   (vanishes! CODEHASH_HI)
   (vanishes! CODEHASH_LO)
   (vanishes! IS_PUSH)
   (vanishes! IS_PUSH_DATA)
   (vanishes! PUSH_PARAMETER)
   (vanishes! PUSH_PARAMETER_OFFSET)
   (vanishes! PUSH_VALUE_HI)
   (vanishes! PUSH_VALUE_LO)
   (vanishes! PUSH_VALUE_ACC_HI)
   (vanishes! PUSH_VALUE_ACC_LO)
   (vanishes! PUSH_FUNNEL_BIT)
   (vanishes! PADDED_BYTECODE_BYTE)
   (vanishes! OPCODE)
   (vanishes! PADDING_BIT)
   (vanishes! PC)
   (vanishes! CODESIZE_REACHED)
   (vanishes! IS_BYTECODE)))

;; ct <=> COUNTER
;; cb <=> CYCLIC_BIT
;; X  <=> some column
;; X should stay constant on cycles like so:
;; ct:  ? ? ? 0 1 2 ... 14 15 0 1 2 ... 14 15 ? ? ?
;; cb:  ? ? ? 0 0 0 ...  0  0 1 1 1 ...  1  1 ? ? ?
;; X :  ? ? ? x x x ...  x  x x x x ...  x  x ? ? ?
(defpurefun (fully-counter-constant ct cb X)
  (if-zero ct
           ;; ct == 0
           (if-not-zero cb (remained-constant! X))
           ;; ct != 0
           (remained-constant! X)))

(defconstraint constancies (:guard (not-in-padding))
  (begin
   ;; CYCLIC_BIT is counter constant
   (if-not-zero (eq! COUNTER 15) (will-remain-constant! CYCLIC_BIT))
   ;; TODO @Olivier
   ;; CODE_FRAGMENT_INDEX & PADDING_BIT are fully counter constant
   (fully-counter-constant COUNTER CYCLIC_BIT CODE_FRAGMENT_INDEX)
   (fully-counter-constant COUNTER CYCLIC_BIT PADDING_BIT)
   ;; ADDRESS_INDEX,IS_INITCODE,CODESIZE,CODEHASH_HI, CODEHASH_LO are constant w.r.t. CODE_FRAGMENT_INDEX
   (stamp-constancy CODE_FRAGMENT_INDEX ADDRESS_INDEX)
   (stamp-constancy CODE_FRAGMENT_INDEX IS_INITCODE)
   (stamp-constancy CODE_FRAGMENT_INDEX CODESIZE)
   (stamp-constancy CODE_FRAGMENT_INDEX CODEHASH_HI)
   (stamp-constancy CODE_FRAGMENT_INDEX CODEHASH_LO)))

(defconstraint automatic (:guard (not-in-padding))
  (begin (if-zero CODESIZE
                  (begin (eq! CODEHASH_HI EMPTY_CODE_HASH_HI)
                         (eq! CODEHASH_LO EMPTY_CODE_HASH_LO)))
         (if-zero (all! (eq! CODEHASH_HI EMPTY_CODE_HASH_HI)
                        (eq! CODEHASH_LO EMPTY_CODE_HASH_LO))
                  (vanishes! CODESIZE))))

(defun (flip x)
    (will-eq! x (- 1 x)))

(defconstraint counter (:guard (not-in-padding))
  (if-zero (eq! COUNTER 15)
           (vanishes! (next COUNTER))
           (will-inc! COUNTER 1)))

(defconstraint cyclic-bit (:guard (not-in-padding))
  (if-zero (eq! COUNTER 15)
           (flip CYCLIC_BIT)))

(defconstraint address-index (:guard (not-in-padding))
  (begin (* (will-remain-constant! ADDRESS_INDEX)
            (will-inc! ADDRESS_INDEX 1))
         (if-zero (will-remain-constant! SC_ADDRESS_HI)
                  (if-zero (will-remain-constant! SC_ADDRESS_LO)
                           (will-remain-constant! ADDRESS_INDEX)))
         (if-zero (will-remain-constant! ADDRESS_INDEX)
                  (begin (will-remain-constant! SC_ADDRESS_LO)
                         (will-remain-constant! SC_ADDRESS_HI)))))

(defconstraint is-initcode (:guard (not-in-padding))
  (if-zero (will-remain-constant! ADDRESS_INDEX)
           (if-not-zero (will-remain-constant! IS_INITCODE)
                        (begin (eq! IS_INITCODE 1)
                               (vanishes! (next IS_INITCODE))))))

(defconstraint code-fragment-index (:guard (not-in-padding))
  (if-zero (will-remain-constant! ADDRESS_INDEX)
           (if-zero (will-remain-constant! IS_INITCODE)
                    (will-remain-constant! CODE_FRAGMENT_INDEX)
                    (will-inc! CODE_FRAGMENT_INDEX 1))
           (will-inc! CODE_FRAGMENT_INDEX 1)))

(defconstraint load-and-initcode (:guard (not-in-padding))
  (if-zero (eq! COUNTER 64)
           (if-zero (eq! CYCLIC_BIT 1)
                    (if-zero (will-remain-constant! ADDRESS_INDEX)
                             (* (will-remain-constant! IS_INITCODE)
                                (+ 1 (will-remain-constant! IS_INITCODE))))))) ;; the fuck is that...

(defconstraint pc (:guard (not-in-padding))
  (if-zero (will-remain-constant! CODE_FRAGMENT_INDEX)
           (will-inc! PC 1)
           (vanishes! (next PC))))

(defconstraint codesize-reached (:guard (not-in-padding))
  (if-zero (will-remain-constant! CODE_FRAGMENT_INDEX)
           (begin (* (will-remain-constant! CODESIZE_REACHED) (- (will-remain-constant! CODESIZE_REACHED) 1))
                  (if-zero (next (- CODESIZE (+ PC 1)))
                           (will-eq! CODESIZE_REACHED (+ CODESIZE_REACHED 1))
                           (will-remain-constant! CODESIZE_REACHED)))
           ;; Seemingly buggy for now TODO @Olivier
           ;; (if-zero (next IS_LOADED)
           ;;          (vanishes! (next CODESIZE_REACHED))
           ;;          (if-zero (- (next CODESIZE) 1)
           ;;                   (vanishes! (- (next CODESIZE_REACHED) 1))
           ;;                   (vanishes! (next CODESIZE_REACHED))))
           0))

(defconstraint padding-bit (:guard (not-in-padding))
  (begin
   (if-not-zero (will-remain-constant! CODE_FRAGMENT_INDEX)
                (vanishes! (next (if-not-zero CODESIZE
                                             (- PADDING_BIT 1)
                                             PADDING_BIT))))
   (if-zero (eq! COUNTER 15)
            (if-zero (eq! CYCLIC_BIT 1)
                     (begin (if-zero CODESIZE_REACHED
                                     (eq! PADDING_BIT 1)
                                     (if-zero (eq! PADDING_BIT 1)
                                              (vanishes! (next PADDING_BIT))))
                            (if-zero PADDING_BIT
                                     (will-inc! CODE_FRAGMENT_INDEX 1)))))))

(defconstraint is-bytecode (:guard (not-in-padding))
  (begin
   (if-zero (will-remain-constant! CODE_FRAGMENT_INDEX)
            (will-eq! IS_BYTECODE (- 1 CODESIZE_REACHED))
            (if-zero (next CODESIZE)
                     (vanishes! (next IS_BYTECODE))
                     (will-eq! IS_BYTECODE 1)))
   (if-zero IS_BYTECODE
            (begin (vanishes! OPCODE)
                   (vanishes! PADDED_BYTECODE_BYTE)))))

(defconstraint push-funnel-bit (:guard (not-in-padding))
  (if-zero IS_PUSH_DATA
           (vanishes! PUSH_FUNNEL_BIT)
           (begin (if-zero PUSH_PARAMETER_OFFSET
                           (vanishes! PUSH_FUNNEL_BIT))
                  (if-zero (eq! PUSH_PARAMETER_OFFSET 16)
                           (will-dec! PUSH_FUNNEL_BIT 1)))))

(defun (same-code-fragment-not-pushdata)
    (begin
     (eq! OPCODE PADDED_BYTECODE_BYTE)
     (if-zero IS_PUSH
              (begin (vanishes! PUSH_VALUE_HI)
                     (vanishes! PUSH_VALUE_LO)
                     (vanishes! PUSH_VALUE_ACC_HI)
                     (vanishes! PUSH_VALUE_ACC_LO)
                     (vanishes! PUSH_PARAMETER_OFFSET)
                     (vanishes! (next IS_PUSH_DATA)))
              (begin (will-eq! IS_PUSH_DATA 1)
                     (will-remain-constant! PUSH_VALUE_HI)
                     (will-remain-constant! PUSH_VALUE_LO)
                     (vanishes! PUSH_VALUE_ACC_HI)
                     (vanishes! PUSH_VALUE_ACC_LO)
                     (eq! PUSH_PARAMETER_OFFSET PUSH_PARAMETER)
                     (will-eq! PUSH_PARAMETER_OFFSET (- PUSH_PARAMETER 1))))))

(defun (same-code-fragment-pushdata)
    (begin
     ;; opcode is zero in push data
     (vanishes! OPCODE)
     ;; the push value is built from the accumulators
     (if-zero PUSH_FUNNEL_BIT
              (begin (remained-constant! PUSH_VALUE_ACC_HI)
                     (eq! PUSH_VALUE_ACC_LO (+ PADDED_BYTECODE_BYTE
                                              (* 256 (prev PUSH_VALUE_ACC_LO)))))
              (begin (remained-constant! PUSH_VALUE_ACC_LO)
                     (eq! PUSH_VALUE_ACC_HI (+ PADDED_BYTECODE_BYTE
                                              (* 256 (prev PUSH_VALUE_ACC_HI))))))
     ;; skim through the push data
     (if-zero PUSH_PARAMETER_OFFSET
              (begin (vanishes! (next IS_PUSH_DATA))
                     (eq! PUSH_VALUE_HI PUSH_VALUE_ACC_HI)
                     (eq! PUSH_VALUE_LO PUSH_VALUE_ACC_LO))
              (begin (will-dec! PUSH_PARAMETER_OFFSET 1)
                     (will-eq! IS_PUSH_DATA 1)
                     (will-remain-constant! PUSH_VALUE_HI)
                     (will-remain-constant! PUSH_VALUE_LO)))))

(defun (same-code-fragment)
    (if-zero IS_PUSH_DATA
             (same-code-fragment-not-pushdata)
             (same-code-fragment-pushdata)))

(defconstraint push (:guard (not-in-padding))
  (if-zero (will-remain-constant! CODE_FRAGMENT_INDEX)
           (begin (will-inc! PC 1)
                  (same-code-fragment))
           (begin (vanishes! IS_PUSH_DATA)
                  (vanishes! (next (- OPCODE PADDED_BYTECODE_BYTE))))))
