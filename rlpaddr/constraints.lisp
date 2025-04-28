(module rlpaddr)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    3.1 Heartbeat             ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint no-stamp-no-things ()
  (if-zero (force-bin (+ RECIPE_1 RECIPE_2))
           (begin (debug (vanishes! ct))
                  (vanishes! ADDR_HI)
                  (vanishes! ADDR_LO)
                  (vanishes! DEP_ADDR_HI)
                  (vanishes! DEP_ADDR_LO)
                  (vanishes! RAW_ADDR_HI)
                  (debug (vanishes! SALT_HI))
                  (debug (vanishes! SALT_LO))
                  (debug (vanishes! NONCE))
                  (debug (vanishes! BYTE1))
                  (debug (vanishes! BIT1)))))

(defconstraint stamp-increments ()
  (or! (eq! STAMP (prev STAMP))
       (eq! STAMP (+ (prev STAMP) 1))))

(defconstraint ct-reset ()
  (if-not (remained-constant! STAMP)
          (vanishes! ct)))

(defconstraint ct-increment ()
  (begin (if-eq RECIPE_1 1
                (if-eq-else ct MAX_CT_CREATE (will-inc! STAMP 1) (will-inc! ct 1)))
         (if-eq RECIPE_2 1
                (if-eq-else ct MAX_CT_CREATE2 (will-inc! STAMP 1) (will-inc! ct 1)))))

(defconstraint last-row (:domain {-1})
  (begin (if-eq RECIPE_1 1 (eq! ct MAX_CT_CREATE))
         (if-eq RECIPE_2 1 (eq! ct MAX_CT_CREATE2))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.2 Constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint stamp-constancies ()
  (begin (stamp-constancy STAMP ADDR_HI)
         (stamp-constancy STAMP ADDR_LO)
         (stamp-constancy STAMP DEP_ADDR_HI)
         (stamp-constancy STAMP DEP_ADDR_LO)
         (stamp-constancy STAMP RAW_ADDR_HI)
         (stamp-constancy STAMP NONCE)
         (stamp-constancy STAMP SALT_HI)
         (stamp-constancy STAMP SALT_LO)
         (stamp-constancy STAMP KEC_HI)
         (stamp-constancy STAMP KEC_LO)
         (stamp-constancy STAMP RECIPE)
         (stamp-constancy STAMP TINY_NON_ZERO_NONCE)))

(defpurefun (ct-incrementing ct X)
  (if-not-zero ct
               (or! (remained-constant! X)
                    (did-inc! X 1))))

(defconstraint ct-incrementings ()
  (begin (ct-incrementing ct INDEX)
         (ct-incrementing ct LC)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    3.4 Byte and Bit decomposition   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint byte-bit-decomposition ()
  (if-zero ct
           (begin (eq! ACC BYTE1)
                  (eq! BIT_ACC BIT1))
           (begin (eq! ACC
                       (+ (* 256 (prev ACC))
                          BYTE1))
                  (eq! BIT_ACC
                       (+ (* 2 (prev BIT_ACC))
                          BIT1)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    3.5 Recipe constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint recipe-addition ()
  (if-zero STAMP
           (vanishes! (+ RECIPE_1 RECIPE_2))
           (eq! 1 (+ RECIPE_1 RECIPE_2))))

(defconstraint setting-recipe-flag ()
  (eq! RECIPE
       (+ (* RLP_ADDR_RECIPE_1 RECIPE_1) (* RLP_ADDR_RECIPE_2 RECIPE_2))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    3.6 SELECTOR_KECCAK_RES   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-kec-res-selector ()
  (eq! SELECTOR_KECCAK_RES
       (* (~ STAMP)
          (- STAMP (prev STAMP)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    4.1 RECIPE_1 constraints  ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint recipe1-byte-decomposition (:guard RECIPE_1)
  (if-zero ct
           (if-zero ACC
                    (begin (vanishes! ACC_BYTESIZE)
                           (eq! POWER (^ 256 8)))
                    (begin (eq! ACC_BYTESIZE 1)
                           (eq! POWER (^ 256 7))))
           (if-zero ACC
                    (begin (remained-constant! ACC_BYTESIZE)
                           (eq! POWER
                                (* (prev POWER) 256)))
                    (begin (did-inc! ACC_BYTESIZE 1)
                           (remained-constant! POWER)))))

(defconstraint recipe1-last-row (:guard RECIPE_1)
  (if-eq ct MAX_CT_CREATE
         (begin (vanishes! (shift INDEX -7))
                (eq! ACC NONCE)
                (eq! BIT_ACC BYTE1)
                (if (and! (eq! ACC_BYTESIZE 1) (eq! (shift BIT1 -7) 0))
                          (eq! 1 TINY_NON_ZERO_NONCE)
                          (vanishes! TINY_NON_ZERO_NONCE))
                (eq! (+ (shift LC -4) (shift LC -3))
                     1)
                (eq! (shift LIMB -3)
                     (* (+ RLP_PREFIX_LIST_SHORT 1 20 ACC_BYTESIZE (- 1 TINY_NON_ZERO_NONCE))
                        (^ 256 15)))
                (eq! (shift nBYTES -3) 1)
                (vanishes! (shift INDEX -3))
                (eq! (shift LIMB -2)
                     (+ (* (+ RLP_PREFIX_INT_SHORT 20) (^ 256 15))
                        (* ADDR_HI (^ 256 11))))
                (eq! (shift nBYTES -2) 5)
                (eq! (prev LIMB) ADDR_LO)
                (eq! (prev nBYTES) 16)
                (if-zero NONCE
                         (eq! LIMB
                              (* RLP_PREFIX_INT_SHORT (^ 256 15)))
                         (if-eq-else 1 TINY_NON_ZERO_NONCE
                                     (eq! LIMB
                                          (* NONCE (^ 256 15)))
                                     (eq! LIMB
                                          (+ (* (+ RLP_PREFIX_INT_SHORT ACC_BYTESIZE) (^ 256 15))
                                             (* NONCE POWER)))))
                (eq! nBYTES
                     (+ ACC_BYTESIZE (- 1 TINY_NON_ZERO_NONCE)))
                (eq! INDEX 3))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    6 RECIPIE2 constraints    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint index-create2 (:guard RECIPE_2)
  (eq! INDEX ct))

(defconstraint create2-last-row (:guard RECIPE_2)
  (if-eq ct MAX_CT_CREATE2
         (begin (eq! (shift LC -5) 1)
                (eq! (shift LIMB -5)
                     (+ (* CREATE2_SHIFT (^ 256 15))
                        (* ADDR_HI (^ 256 11))))
                (eq! (shift nBYTES -5) 5)
                (eq! (shift LIMB -4) ADDR_LO)
                (eq! (shift nBYTES -4) LLARGE)
                (eq! (shift LIMB -3) SALT_HI)
                (eq! (shift nBYTES -3) LLARGE)
                (eq! (shift LIMB -2) SALT_LO)
                (eq! (shift nBYTES -2) LLARGE)
                (eq! (prev LIMB) KEC_HI)
                (eq! (prev nBYTES) LLARGE)
                (eq! LIMB KEC_LO)
                (eq! nBYTES LLARGE))))


