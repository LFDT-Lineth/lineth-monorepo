(module oob)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;   2 Constraints     ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; 2.1 shorthands and  ;;
;;     constants       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (flag-sum-inst)                      (+    IS_JUMP IS_JUMPI
                                                  IS_RDC
                                                  IS_CDL
                                                  IS_XCALL
                                                  IS_CALL
                                                  IS_CREATE
                                                  IS_SSTORE
                                                  IS_DEPLOYMENT))

(defun (flag-sum-prc-common)                (+    IS_ECRECOVER
                                                  IS_SHA2
                                                  IS_RIPEMD
                                                  IS_IDENTITY
                                                  IS_ECADD
                                                  IS_ECMUL
                                                  IS_ECPAIRING))

(defun (flag-sum-prc-blake)                 (+    IS_BLAKE2F_CDS
                                                  IS_BLAKE2F_PARAMS))

(defun (flag-sum-prc-modexp)                (+    IS_MODEXP_CDS
                                                  IS_MODEXP_XBS
                                                  IS_MODEXP_LEAD
                                                  IS_MODEXP_PRICING
                                                  IS_MODEXP_EXTRACT))

(defun (flag-sum-prc)                       (+    (flag-sum-prc-common)
                                                  (flag-sum-prc-blake)
                                                  (flag-sum-prc-modexp)))

(defun (flag-sum)                           (+    (flag-sum-inst)
                                                  (flag-sum-prc)))

(defun (wght-sum-inst)                      (+    (* OOB_INST_JUMP             IS_JUMP)
                                                  (* OOB_INST_JUMPI            IS_JUMPI)
                                                  (* OOB_INST_RDC              IS_RDC)
                                                  (* OOB_INST_CDL              IS_CDL)
                                                  (* OOB_INST_XCALL            IS_XCALL)
                                                  (* OOB_INST_CALL             IS_CALL)
                                                  (* OOB_INST_CREATE           IS_CREATE)
                                                  (* OOB_INST_SSTORE           IS_SSTORE)
                                                  (* OOB_INST_DEPLOYMENT       IS_DEPLOYMENT)))

(defun (wght-sum-prc-common)                (+    (* OOB_INST_ECRECOVER        IS_ECRECOVER)
                                                  (* OOB_INST_SHA2             IS_SHA2)
                                                  (* OOB_INST_RIPEMD           IS_RIPEMD)
                                                  (* OOB_INST_IDENTITY         IS_IDENTITY)
                                                  (* OOB_INST_ECADD            IS_ECADD)
                                                  (* OOB_INST_ECMUL            IS_ECMUL)
                                                  (* OOB_INST_ECPAIRING        IS_ECPAIRING)))

(defun (wght-sum-prc-blake)                 (+    (* OOB_INST_BLAKE_CDS        IS_BLAKE2F_CDS)
                                                  (* OOB_INST_BLAKE_PARAMS     IS_BLAKE2F_PARAMS)))

(defun (wght-sum-prc-modexp)                (+    (* OOB_INST_MODEXP_CDS       IS_MODEXP_CDS)
                                                  (* OOB_INST_MODEXP_XBS       IS_MODEXP_XBS)
                                                  (* OOB_INST_MODEXP_LEAD      IS_MODEXP_LEAD)
                                                  (* OOB_INST_MODEXP_PRICING   IS_MODEXP_PRICING)
                                                  (* OOB_INST_MODEXP_EXTRACT   IS_MODEXP_EXTRACT)))

(defun (wght-sum-prc)                       (+    (wght-sum-prc-common)
                                                  (wght-sum-prc-blake)
                                                  (wght-sum-prc-modexp)))

(defun (wght-sum)                           (+    (wght-sum-inst)
                                                  (wght-sum-prc)))

(defun (maxct-sum-inst)                     (+    (* CT_MAX_JUMP               IS_JUMP)
                                                  (* CT_MAX_JUMPI              IS_JUMPI)
                                                  (* CT_MAX_RDC                IS_RDC)
                                                  (* CT_MAX_CDL                IS_CDL)
                                                  (* CT_MAX_XCALL              IS_XCALL)
                                                  (* CT_MAX_CALL               IS_CALL)
                                                  (* CT_MAX_CREATE             IS_CREATE)
                                                  (* CT_MAX_SSTORE             IS_SSTORE)
                                                  (* CT_MAX_DEPLOYMENT         IS_DEPLOYMENT)))

(defun (maxct-sum-prc-common)               (+    (* CT_MAX_ECRECOVER          IS_ECRECOVER)
                                                  (* CT_MAX_SHA2               IS_SHA2)
                                                  (* CT_MAX_RIPEMD             IS_RIPEMD)
                                                  (* CT_MAX_IDENTITY           IS_IDENTITY)
                                                  (* CT_MAX_ECADD              IS_ECADD)
                                                  (* CT_MAX_ECMUL              IS_ECMUL)
                                                  (* CT_MAX_ECPAIRING          IS_ECPAIRING)))

(defun (maxct-sum-prc-blake)                (+    (* CT_MAX_BLAKE2F_CDS IS_BLAKE2F_CDS)
                                                  (* CT_MAX_BLAKE2F_PARAMS IS_BLAKE2F_PARAMS)))

(defun (maxct-sum-prc-modexp)               (+    (* CT_MAX_MODEXP_CDS IS_MODEXP_CDS)
                                                  (* CT_MAX_MODEXP_XBS IS_MODEXP_XBS)
                                                  (* CT_MAX_MODEXP_LEAD IS_MODEXP_LEAD)
                                                  (* CT_MAX_MODEXP_PRICING IS_MODEXP_PRICING)
                                                  (* CT_MAX_MODEXP_EXTRACT IS_MODEXP_EXTRACT)))

(defun (maxct-sum-prc)                      (+    (maxct-sum-prc-common)
                                                  (maxct-sum-prc-blake)
                                                  (maxct-sum-prc-modexp)))

(defun (maxct-sum)                          (+    (maxct-sum-inst)
                                                  (maxct-sum-prc)))

(defun (lookup-sum k)                       (+    (shift ADD_FLAG k)
                                                  (shift MOD_FLAG k)
                                                  (shift WCP_FLAG k)))

(defun (wght-lookup-sum k)                  (+    (* 1 (shift ADD_FLAG k))
                                                  (* 2 (shift MOD_FLAG k))
                                                  (* 3 (shift WCP_FLAG k))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 binary constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint wcp-add-mod-are-exclusive ()
  (is-binary (lookup-sum 0)))

;; others are done with binary@prove in columns.lisp

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;    2.3 instruction decoding   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint flag-sum-vanishes ()
  (if-zero STAMP
           (vanishes! (flag-sum))))

(defconstraint flag-sum-equal-one ()
  (if-not-zero STAMP
               (eq! (flag-sum) 1)))

(defconstraint decoding ()
  (eq! OOB_INST (wght-sum)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.4 Constancy            ;;
;;        constraints          ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint counter-constancy ()
  (begin (counter-constancy CT STAMP)
         (debug (counter-constancy CT CT_MAX))
         (for i [9] (counter-constancy CT [DATA i]))
         (counter-constancy CT OOB_INST)))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.5 heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first-row (:domain {0})
  (vanishes! STAMP))

(defconstraint padding-vanishing ()
  (if-zero STAMP
           (begin (vanishes! CT)
                  (vanishes! (+ (lookup-sum 0) (flag-sum))))))

(defconstraint stamp-increments ()
  (any! (remained-constant! STAMP) (did-inc! STAMP 1)))

(defconstraint counter-reset ()
  (if-not-zero (remained-constant! STAMP)
               (vanishes! CT)))

(defconstraint ct-max ()
  (eq! CT_MAX (maxct-sum)))

(defconstraint non-trivial-instruction-counter-cycle ()
  (if-not-zero STAMP
               (if-eq-else CT CT_MAX (will-inc! STAMP 1) (will-inc! CT 1))))

(defconstraint final-row (:domain {-1})
  (if-not-zero STAMP
               (eq! CT CT_MAX)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.6 Constraint systems   ;;
;;    for populating lookups   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; support function to improve to reduce code duplication in the functions below
(defun (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (call-to-ADD k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 1)
         (eq! (shift OUTGOING_INST k) EVM_INST_ADD)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-DIV k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 2)
         (eq! (shift OUTGOING_INST k) EVM_INST_DIV)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-MOD k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 2)
         (eq! (shift OUTGOING_INST k) EVM_INST_MOD)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-LT k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_LT)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-GT k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_GT)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-EQ k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_EQ)
         (set-args k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)))

(defun (call-to-ISZERO k arg_1_hi arg_1_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_ISZERO)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (debug (vanishes! (shift [OUTGOING_DATA 3] k)))
         (debug (vanishes! (shift [OUTGOING_DATA 4] k)))))

(defun (noCall k)
  (begin (eq! (wght-lookup-sum k) 0)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;  3 Populating opcodes     ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (assumption---fresh-new-stamp) (- STAMP (prev STAMP)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.3 For JUMP       ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (jump---standard-precondition)       IS_JUMP)
(defun (jump---pc-new-hi)                   [DATA 1])
(defun (jump---pc-new-lo)                   [DATA 2])
(defun (jump---code-size)                   [DATA 5])
(defun (jump---guaranteed-exception)        [DATA 7])
(defun (jump---jump-must-be-attempted)      [DATA 8])
(defun (jump---valid-pc-new)                OUTGOING_RES_LO)

(defconstraint jump---compare-pc-new-against-code-size (:guard (* (assumption---fresh-new-stamp) (jump---standard-precondition)))
  (call-to-LT 0 (jump---pc-new-hi) (jump---pc-new-lo) 0 (jump---code-size)))

(defconstraint jump---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (jump---standard-precondition)))
  (begin (eq! (jump---guaranteed-exception) (- 1 (jump---valid-pc-new)))
         (eq! (jump---jump-must-be-attempted) (jump---valid-pc-new))
         (debug (is-binary (jump---guaranteed-exception)))
         (debug (is-binary (jump---jump-must-be-attempted)))
         (debug (eq! (+ (jump---guaranteed-exception) (jump---jump-must-be-attempted)) 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.4 For JUMPI      ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (jumpi---standard-precondition)      IS_JUMPI)
(defun (jumpi---pc-new-hi)                  [DATA 1])
(defun (jumpi---pc-new-lo)                  [DATA 2])
(defun (jumpi---jump-cond-hi)               [DATA 3])
(defun (jumpi---jump-cond-lo)               [DATA 4])
(defun (jumpi---code-size)                  [DATA 5])
(defun (jumpi---jump-not-attempted)         [DATA 6])
(defun (jumpi---guaranteed-exception)       [DATA 7])
(defun (jumpi---jump-must-be-attempted)     [DATA 8])
(defun (jumpi---valid-pc-new)               OUTGOING_RES_LO)
(defun (jumpi---jump-cond-is-zero)          (next OUTGOING_RES_LO))

(defconstraint jumpi---compare-pc-new-against-code-size (:guard (* (assumption---fresh-new-stamp) (jumpi---standard-precondition)))
  (call-to-LT 0 (jumpi---pc-new-hi) (jumpi---pc-new-lo) 0 (jumpi---code-size)))

(defconstraint jumpi---check-jump-cond-is-zero (:guard (* (assumption---fresh-new-stamp) (jumpi---standard-precondition)))
  (call-to-ISZERO 1 (jumpi---jump-cond-hi) (jumpi---jump-cond-lo)))

(defconstraint jumpi---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (jumpi---standard-precondition)))
  (begin (eq! (jumpi---jump-not-attempted) (jumpi---jump-cond-is-zero))
         (eq! (jumpi---guaranteed-exception)
              (* (- 1 (jumpi---jump-cond-is-zero)) (- 1 (jumpi---valid-pc-new))))
         (eq! (jumpi---jump-must-be-attempted)
              (* (- 1 (jumpi---jump-cond-is-zero)) (jumpi---valid-pc-new)))
         (debug (is-binary (jumpi---jump-not-attempted)))
         (debug (is-binary (jumpi---guaranteed-exception)))
         (debug (is-binary (jumpi---jump-must-be-attempted)))
         (debug (eq! (+ (jumpi---guaranteed-exception)
                        (jumpi---jump-must-be-attempted)
                        (jumpi---jump-not-attempted))
                     1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.5 For               ;;
;; RETURNDATACOPY        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Note. We use rdc as a shorthand for RETURNDATACOPY

(defun (rdc---standard-precondition)    IS_RDC)
(defun (rdc---offset-hi)                [DATA 1])
(defun (rdc---offset-lo)                [DATA 2])
(defun (rdc---size-hi)                  [DATA 3])
(defun (rdc---size-lo)                  [DATA 4])
(defun (rdc---rds)                      [DATA 5])
(defun (rdc---rdcx)                     [DATA 7])
(defun (rdc---rdc-roob)                 (- 1 OUTGOING_RES_LO))
(defun (rdc---rdc-soob)                 (shift OUTGOING_RES_LO 2))

(defconstraint rdc---check-offset-is-zero (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (call-to-ISZERO 0 (rdc---offset-hi) (rdc---size-hi)))

(defconstraint rdc---add-offset-and-size (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (if-zero (rdc---rdc-roob)
           (call-to-ADD 1 0 (rdc---offset-lo) 0 (rdc---size-lo))
           (noCall 1)))

(defconstraint rdc---compare-offset-plus-size-against-rds (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (if-zero (rdc---rdc-roob)
           (begin (vanishes! (shift ADD_FLAG 2))
                  (vanishes! (shift MOD_FLAG 2))
                  (eq! (shift WCP_FLAG 2) 1)
                  (eq! (shift OUTGOING_INST 2) EVM_INST_GT)
                  (vanishes! (shift [OUTGOING_DATA 3] 2))
                  (eq! (shift [OUTGOING_DATA 4] 2) (rdc---rds)))
           (noCall 2)))

(defconstraint rdc---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (rdc---standard-precondition)))
  (eq! (rdc---rdcx)
       (+ (rdc---rdc-roob)
          (* (- 1 (rdc---rdc-roob)) (rdc---rdc-soob)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.6 For               ;;
;; CALLDATALOAD          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Note. We use cdl as a shorthand for CALLDATALOAD

(defun (cdl---standard-precondition)    IS_CDL)
(defun (cdl---offset-hi)                [DATA 1])
(defun (cdl---offset-lo)                [DATA 2])
(defun (cdl---cds)                      [DATA 5])
(defun (cdl---cdl-out-of-bounds)        [DATA 7])
(defun (cdl---touches-ram)              OUTGOING_RES_LO)

(defconstraint cdl---compare-offset-against-cds (:guard (* (assumption---fresh-new-stamp) (cdl---standard-precondition)))
  (call-to-LT 0 (cdl---offset-hi) (cdl---offset-lo) 0 (cdl---cds)))

(defconstraint cdl---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (cdl---standard-precondition)))
  (eq! (cdl---cdl-out-of-bounds) (- 1 (cdl---touches-ram))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.7 For               ;;
;; SSTORE                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (sstore---standard-precondition)     IS_SSTORE)
(defun (sstore---gas)                       [DATA 5])
(defun (sstore---sstorex)                   [DATA 7])
(defun (sstore---sufficient-gas)            OUTGOING_RES_LO)

(defconstraint sstore---compare-g-call-stipend-against-gas (:guard (* (assumption---fresh-new-stamp) (sstore---standard-precondition)))
  (call-to-LT 0 0 GAS_CONST_G_CALL_STIPEND 0 (sstore---gas)))

(defconstraint sstore---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (sstore---standard-precondition)))
  (eq! (sstore---sstorex) (- 1 (sstore---sufficient-gas))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.8 For               ;;
;; DEPLOYMENT            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Note. Here "DEPLOYMENT" refers to the execution of the RETURN opcode in a deployment context

(defun (deployment---standard-precondition)            IS_DEPLOYMENT)
(defun (deployment---code-size-hi)                     [DATA 1])
(defun (deployment---code-size-lo)                     [DATA 2])
(defun (deployment---max-code-size-exception)          [DATA 7])
(defun (deployment---exceeds-max-code-size)            OUTGOING_RES_LO)

(defconstraint deployment---compare-max-code-size-against-code-size (:guard (* (assumption---fresh-new-stamp) (deployment---standard-precondition)))
  (call-to-LT 0 0 MAX_CODE_SIZE (deployment---code-size-hi) (deployment---code-size-lo)))

(defconstraint deployment---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (deployment---standard-precondition)))
  (eq! (deployment---max-code-size-exception) (deployment---exceeds-max-code-size)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.9 For XCALL's    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; Note. We use XCALL as a shorthand for "eXceptional CALL-type instruction"

(defun (xcall---standard-precondition)      IS_XCALL)
(defun (xcall---value-hi)                   [DATA 1])
(defun (xcall---value-lo)                   [DATA 2])
(defun (xcall---value-is-nonzero)           [DATA 7])
(defun (xcall---value-is-zero)              [DATA 8])

(defconstraint xcall---check-value-is-zero (:guard (* (assumption---fresh-new-stamp) (xcall---standard-precondition)))
  (call-to-ISZERO 0 (xcall---value-hi) (xcall---value-lo)))

(defconstraint xcall---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (xcall---standard-precondition)))
  (begin (eq! (xcall---value-is-nonzero) (- 1 OUTGOING_RES_LO))
         (eq! (xcall---value-is-zero) OUTGOING_RES_LO)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.10 For CALL's    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (call---standard-precondition)           IS_CALL)
(defun (call---value-hi)                        [DATA 1])
(defun (call---value-lo)                        [DATA 2])
(defun (call---balance)                         [DATA 3])
(defun (call---call-stack-depth)                [DATA 6])
(defun (call---value-is-nonzero)                [DATA 7])
(defun (call---aborting-condition)              [DATA 8])
(defun (call---insufficient-balance-abort)      OUTGOING_RES_LO)
(defun (call---call-stack-depth-abort)          (- 1 (next OUTGOING_RES_LO)))
(defun (call---value-is-zero)                   (shift OUTGOING_RES_LO 2))

(defconstraint call---compare-balance-against-value (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (call-to-LT 0 0 (call---balance) (call---value-hi) (call---value-lo)))

(defconstraint call---compare-call-stack-depth-against-1024 (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (call-to-LT 1 0 (call---call-stack-depth) 0 1024))

(defconstraint call---check-value-is-zero (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (call-to-ISZERO 2 (call---value-hi) (call---value-lo)))

(defconstraint call---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (call---standard-precondition)))
  (begin (eq! (call---value-is-nonzero) (- 1 (call---value-is-zero)))
         (eq! (call---aborting-condition)
              (+ (call---insufficient-balance-abort)
                 (* (- 1 (call---insufficient-balance-abort)) (call---call-stack-depth-abort))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.11 For              ;;
;; CREATE's              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (create---standard-precondition)                IS_CREATE)
(defun (create---value-hi)                             [DATA 1])
(defun (create---value-lo)                             [DATA 2])
(defun (create---balance)                              [DATA 3])
(defun (create---nonce)                                [DATA 4])
(defun (create---has-code)                             [DATA 5])
(defun (create---call-stack-depth)                     [DATA 6])
(defun (create---aborting-condition)                   [DATA 7])
(defun (create---failure-condition)                    [DATA 8])
(defun (create---creator-nonce)                        [DATA 9])
(defun (create---insufficient-balance-abort)           OUTGOING_RES_LO)
(defun (create---stack-depth-abort)                    (- 1 (next OUTGOING_RES_LO)))
(defun (create---nonzero-nonce)                        (- 1 (shift OUTGOING_RES_LO 2)))
(defun (create---creator-nonce-abort)                  (- 1 (shift OUTGOING_RES_LO 3)))
(defun (create---aborting-conditions-sum)              (+ (create---insufficient-balance-abort) (create---stack-depth-abort) (create---creator-nonce-abort)))

(defconstraint create---compare-balance-against-value (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
  (call-to-LT 0 0 (create---balance) (create---value-hi) (create---value-lo)))

(defconstraint create---compare-call-stack-depth-against-1024 (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
  (call-to-LT 1 0 (create---call-stack-depth) 0 1024))

(defconstraint create---check-nonce-is-zero (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
  (call-to-ISZERO 2 0 (create---nonce)))

(defconstraint create---compare-creator-nonce-against-max-nonce (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
  (call-to-LT 3 0 (create---creator-nonce) 0 EIP2681_MAX_NONCE))

(defconstraint create---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (create---standard-precondition)))
  (begin (if-zero (create---aborting-conditions-sum)
                  (vanishes! (create---aborting-condition))
                  (eq! (create---aborting-condition) 1))
         (eq! (create---failure-condition)
              (* (- 1 (create---aborting-condition))
                 (+ (create---has-code)
                    (* (- 1 (create---has-code)) (create---nonzero-nonce)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;   5 Populating common         ;;
;;   precompiles                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 5.1 Common            ;;
;; constraints for       ;;
;; precompiles           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-common---standard-precondition)   (flag-sum-prc-common))
(defun (prc---callee-gas)                     [DATA 1])
(defun (prc---cds)                            [DATA 2])
(defun (prc---r@c)                            [DATA 3])
(defun (prc---hub-success)                    [DATA 4])
(defun (prc---ram-success)                    [DATA 4])
(defun (prc---return-gas)                     [DATA 5])
(defun (prc---extract-call-data)              [DATA 6])
(defun (prc---empty-call-data)                [DATA 7])
(defun (prc---r@c-nonzero)                    [DATA 8])
(defun (prc---cds-is-zero)                    OUTGOING_RES_LO)
(defun (prc---r@c-is-zero)                    (next OUTGOING_RES_LO))

(defconstraint prc---check-cds-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-common---standard-precondition)))
  (call-to-ISZERO 0 0 (prc---cds)))

(defconstraint prc---check-r@c-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-common---standard-precondition)))
  (call-to-ISZERO 1 0 (prc---r@c)))

(defconstraint prc---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-common---standard-precondition)))
  (begin (eq! (prc---extract-call-data)
              (* (prc---hub-success) (- 1 (prc---cds-is-zero))))
         (eq! (prc---empty-call-data) (* (prc---hub-success) (prc---cds-is-zero)))
         (eq! (prc---r@c-nonzero) (- 1 (prc---r@c-is-zero)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 5.2 For ECRECOVER,    ;;
;; ECADD, ECMUL          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-ecrecover-prc-ecadd-prc-ecmul---standard-precondition)    (+ IS_ECRECOVER IS_ECADD IS_ECMUL))
(defun (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost)          (+ (* 3000 IS_ECRECOVER) (* 150 IS_ECADD) (* 6000 IS_ECMUL)))
(defun (prc-ecrecover-prc-ecadd-prc-ecmul---insufficient-gas)         (shift OUTGOING_RES_LO 2))

(defconstraint prc-ecrecover-prc-ecadd-prc-ecmul---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-ecrecover-prc-ecadd-prc-ecmul---standard-precondition)))
  (call-to-LT 2 0 (prc---callee-gas) 0 (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost)))

(defconstraint prc-ecrecover-prc-ecadd-prc-ecmul---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-ecrecover-prc-ecadd-prc-ecmul---standard-precondition)))
  (begin (eq! (prc---hub-success) (- 1 (prc-ecrecover-prc-ecadd-prc-ecmul---insufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---callee-gas) (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 5.3 For SHA2-256,     ;;
;; RIPEMD-160, IDENTITY  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-sha2-prc-ripemd-prc-identity---standard-precondition)     (+ IS_SHA2 IS_RIPEMD IS_IDENTITY))
(defun (prc-sha2-prc-ripemd-prc-identity---ceil)                      (shift OUTGOING_RES_LO 2))
(defun (prc-sha2-prc-ripemd-prc-identity---insufficient-gas)          (shift OUTGOING_RES_LO 3))
(defun (prc-sha2-prc-ripemd-prc-identity---precompile-cost)           (*    (+ 5 (prc-sha2-prc-ripemd-prc-identity---ceil))
                                                                            (+ (* 12 IS_SHA2) (* 120 IS_RIPEMD) (* 3 IS_IDENTITY))))

(defconstraint prc-sha2-prc-ripemd-prc-identity---div-cds-plus-31-by-32 (:guard (* (assumption---fresh-new-stamp) (prc-sha2-prc-ripemd-prc-identity---standard-precondition)))
  (call-to-DIV 2 0 (+ (prc---cds) 31) 0 32))

(defconstraint prc-sha2-prc-ripemd-prc-identity---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-sha2-prc-ripemd-prc-identity---standard-precondition)))
  (call-to-LT 3 0 (prc---callee-gas) 0 (prc-sha2-prc-ripemd-prc-identity---precompile-cost)))

(defconstraint prc-sha2-prc-ripemd-prc-identity---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-sha2-prc-ripemd-prc-identity---standard-precondition)))
  (begin (eq! (prc---hub-success) (- 1 (prc-sha2-prc-ripemd-prc-identity---insufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---callee-gas) (prc-sha2-prc-ripemd-prc-identity---precompile-cost))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 4.4 For ECPAIRING     ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-ecpairing---standard-precondition)     IS_ECPAIRING)
(defun (prc-ecpairing---remainder)                 (shift OUTGOING_RES_LO 2))
(defun (prc-ecpairing---is-multiple_192)           (shift OUTGOING_RES_LO 3))
(defun (prc-ecpairing---insufficient-gas)          (shift OUTGOING_RES_LO 4))
(defun (prc-ecpairing---precompile-cost192)        (*    (prc-ecpairing---is-multiple_192)
                                                         (+ (* 45000 192) (* 34000 (prc---cds)))))

(defconstraint prc-ecpairing---mod-cds-by-192 (:guard (* (assumption---fresh-new-stamp) (prc-ecpairing---standard-precondition)))
  (call-to-MOD 2 0 (prc---cds) 0 192))

(defconstraint prc-ecpairing---check-remainder-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-ecpairing---standard-precondition)))
  (call-to-ISZERO 3 0 (prc-ecpairing---remainder)))

(defconstraint prc-ecpairing---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-ecpairing---standard-precondition)))
  (if-zero (prc-ecpairing---is-multiple_192)
           (noCall 4)
           (begin (vanishes! (shift ADD_FLAG 4))
                  (vanishes! (shift MOD_FLAG 4))
                  (eq! (shift WCP_FLAG 4) 1)
                  (eq! (shift OUTGOING_INST 4) EVM_INST_LT)
                  (vanishes! (shift [OUTGOING_DATA 1] 4))
                  (eq! (shift [OUTGOING_DATA 2] 4) (prc---callee-gas))
                  (vanishes! (shift [OUTGOING_DATA 3] 4))
                  (eq! (* (shift [OUTGOING_DATA 4] 4) 192)
                       (prc-ecpairing---precompile-cost192)))))

(defconstraint prc-ecpairing---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-ecpairing---standard-precondition)))
  (begin (eq! (prc---hub-success)
              (* (prc-ecpairing---is-multiple_192) (- 1 (prc-ecpairing---insufficient-gas))))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (* (prc---return-gas) 192)
                       (- (* (prc---callee-gas) 192) (prc-ecpairing---precompile-cost192))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6 Populating MODEXP   ;;
;;   precompiles           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.1 For MODEXP - cds  ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-cds---standard-precondition)    IS_MODEXP_CDS)
(defun (prc-modexp-cds---extract-bbs)              [DATA 3])
(defun (prc-modexp-cds---extract-ebs)              [DATA 4])
(defun (prc-modexp-cds---extract-mbs)              [DATA 5])

(defconstraint prc-modexp-cds---compare-0-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (call-to-LT 0 0 0 0 (prc---cds)))

(defconstraint prc-modexp-cds---compare-32-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (call-to-LT 1 0 32 0 (prc---cds)))

(defconstraint prc-modexp-cds---compare-64-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (call-to-LT 2 0 64 0 (prc---cds)))

(defconstraint prc-modexp-cds---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-cds---standard-precondition)))
  (begin (eq! (prc-modexp-cds---extract-bbs) OUTGOING_RES_LO)
         (eq! (prc-modexp-cds---extract-ebs) (next OUTGOING_RES_LO))
         (eq! (prc-modexp-cds---extract-mbs) (shift OUTGOING_RES_LO 2))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.2 For MODEXP - xbs  ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-xbs---standard-precondition)    IS_MODEXP_XBS)
(defun (prc-modexp-xbs---xbs-hi)                   [DATA 1])
(defun (prc-modexp-xbs---xbs-lo)                   [DATA 2])
(defun (prc-modexp-xbs---ybs-lo)                   [DATA 3])
(defun (prc-modexp-xbs---compute-max)              [DATA 4])
(defun (prc-modexp-xbs---max-xbs-ybs)              [DATA 7])
(defun (prc-modexp-xbs---xbs-nonzero)              [DATA 8])
(defun (prc-modexp-xbs---compo-to_512)             OUTGOING_RES_LO)
(defun (prc-modexp-xbs---comp)                     (next OUTGOING_RES_LO))

(defconstraint prc-modexp-xbs---compare-xbs-hi-against-513 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (call-to-LT 0 (prc-modexp-xbs---xbs-hi) (prc-modexp-xbs---xbs-lo) 0 513))

(defconstraint prc-modexp-xbs---compare-xbs-against-ybs (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (call-to-LT 1 0 (prc-modexp-xbs---xbs-lo) 0 (prc-modexp-xbs---ybs-lo)))

(defconstraint prc-modexp-xbs---check-xbs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (call-to-ISZERO 2 0 (prc-modexp-xbs---xbs-lo)))

(defconstraint additional-prc-modexp-xbs (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (begin (vanishes! (* (prc-modexp-xbs---compute-max) (- 1 (prc-modexp-xbs---compute-max))))
         (eq! (prc-modexp-xbs---compo-to_512) 1)))

(defconstraint prc-modexp-xbs---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-xbs---standard-precondition)))
  (if-zero (prc-modexp-xbs---compute-max)
           (begin (vanishes! (prc-modexp-xbs---max-xbs-ybs))
                  (vanishes! (prc-modexp-xbs---xbs-nonzero)))
           (begin (eq! (prc-modexp-xbs---xbs-nonzero)
                       (- 1 (shift OUTGOING_RES_LO 2)))
                  (if-zero (prc-modexp-xbs---comp)
                           (eq! (prc-modexp-xbs---max-xbs-ybs) (prc-modexp-xbs---xbs-lo))
                           (eq! (prc-modexp-xbs---max-xbs-ybs) (prc-modexp-xbs---ybs-lo))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.3 For MODEXP        ;;
;;   - lead                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-lead---standard-precondition)               IS_MODEXP_LEAD)
(defun (prc-modexp-lead---bbs)                                 [DATA 1])
(defun (prc-modexp-lead---ebs)                                 [DATA 3])
(defun (prc-modexp-lead---load-lead)                           [DATA 4])
(defun (prc-modexp-lead---cds-cutoff)                          [DATA 6])
(defun (prc-modexp-lead---ebs-cutoff)                          [DATA 7])
(defun (prc-modexp-lead---sub-ebs_32)                          [DATA 8])
(defun (prc-modexp-lead---ebs-is-zero)                         OUTGOING_RES_LO)
(defun (prc-modexp-lead---ebs-less-than_32)                    (next OUTGOING_RES_LO))
(defun (prc-modexp-lead---call-data-contains-exponent-bytes)   (shift OUTGOING_RES_LO 2))
(defun (prc-modexp-lead---comp)                                (shift OUTGOING_RES_LO 3))

(defconstraint prc-modexp-lead---check-ebs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (call-to-ISZERO 0 0 (prc-modexp-lead---ebs)))

(defconstraint prc-modexp-lead---compare-ebs-against-32 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (call-to-LT 1 0 (prc-modexp-lead---ebs) 0 32))

(defconstraint prc-modexp-lead---compare-ebs-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (call-to-LT 2 0 (+ 96 (prc-modexp-lead---bbs)) 0 (prc---cds)))

(defconstraint prc-modexp-lead---compare-cds-minus-96-plus-bbs-against-32 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (if-not-zero (prc-modexp-lead---call-data-contains-exponent-bytes)
               (call-to-LT 3
                           0
                           (- (prc---cds) (+ 96 (prc-modexp-lead---bbs)))
                           0
                           32)))

(defconstraint prc-modexp-lead---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-lead---standard-precondition)))
  (begin (eq! (prc-modexp-lead---load-lead)
              (* (prc-modexp-lead---call-data-contains-exponent-bytes)
                 (- 1 (prc-modexp-lead---ebs-is-zero))))
         (if-zero (prc-modexp-lead---call-data-contains-exponent-bytes)
                  (vanishes! (prc-modexp-lead---cds-cutoff))
                  (if-zero (prc-modexp-lead---comp)
                           (eq! (prc-modexp-lead---cds-cutoff) 32)
                           (eq! (prc-modexp-lead---cds-cutoff)
                                (- (prc---cds) (+ 96 (prc-modexp-lead---bbs))))))
         (if-zero (prc-modexp-lead---ebs-less-than_32)
                  (eq! (prc-modexp-lead---ebs-cutoff) 32)
                  (eq! (prc-modexp-lead---ebs-cutoff) (prc-modexp-lead---ebs)))
         (if-zero (prc-modexp-lead---ebs-less-than_32)
                  (eq! (prc-modexp-lead---sub-ebs_32) (- (prc-modexp-lead---ebs) 32))
                  (vanishes! (prc-modexp-lead---sub-ebs_32)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.4 For MODEXP        ;;
;;   - pricing             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-pricing---standard-precondition)   IS_MODEXP_PRICING)
(defun (prc-modexp-pricing---exponent-log)            [DATA 6])
(defun (prc-modexp-pricing---max-xbs-ybs)             [DATA 7])
(defun (prc-modexp-pricing---exponent-log-is-zero)    (next OUTGOING_RES_LO))
(defun (prc-modexp-pricing---f-of-max)                (*  (shift OUTGOING_RES_LO 2)  (shift OUTGOING_RES_LO 2)))
(defun (prc-modexp-pricing---big-quotient)            (shift OUTGOING_RES_LO 3))
(defun (prc-modexp-pricing---big-quotient_LT_200)     (shift OUTGOING_RES_LO 4))
(defun (prc-modexp-pricing---big-numerator)           (if-zero (prc-modexp-pricing---exponent-log-is-zero)
                                                               (* (prc-modexp-pricing---f-of-max) (prc-modexp-pricing---exponent-log))
                                                               (prc-modexp-pricing---f-of-max)))
(defun (prc-modexp-pricing---precompile-cost)         (if-zero (prc-modexp-pricing---big-quotient_LT_200)
                                                               (prc-modexp-pricing---big-quotient)
                                                               200))

(defconstraint prc-modexp-pricing---check--is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-ISZERO 0 0 (prc---r@c)))

(defconstraint prc-modexp-pricing---check-exponent-log-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-ISZERO 1 0 (prc-modexp-pricing---exponent-log)))

(defconstraint prc-modexp-pricing---div-max-xbs-ybs-plus-7-by-8 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-DIV 2
               0
               (+ (prc-modexp-pricing---max-xbs-ybs) 7)
               0
               8))

(defconstraint prc-modexp-pricing---div-big-numerator-by-quaddivisor (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-DIV 3 0 (prc-modexp-pricing---big-numerator) 0 G_QUADDIVISOR))

(defconstraint prc-modexp-pricing---compare-big-quotient-against-200 (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-LT 4 0 (prc-modexp-pricing---big-quotient) 0 200))

(defconstraint prc-modexp-pricing---compare-call-gas-against-precompile-cost (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (call-to-LT 5 0 (prc---callee-gas) 0 (prc-modexp-pricing---precompile-cost)))

(defconstraint prc-modexp-pricing---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-pricing---standard-precondition)))
  (begin (eq! (prc---ram-success)
              (- 1 (shift OUTGOING_RES_LO 5)))
         (if-zero (prc---ram-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas) (- (prc---callee-gas) (prc-modexp-pricing---precompile-cost))))
         (eq! (prc---r@c-nonzero) (- 1 OUTGOING_RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.5 For MODEXP        ;;
;;   - extract             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-modexp-extract---standard-precondition)                 IS_MODEXP_EXTRACT)
(defun (prc-modexp-extract---bbs)                                   [DATA 3])
(defun (prc-modexp-extract---ebs)                                   [DATA 4])
(defun (prc-modexp-extract---mbs)                                   [DATA 5])
(defun (prc-modexp-extract---extract-base)                          [DATA 6])
(defun (prc-modexp-extract---extract-exponent)                      [DATA 7])
(defun (prc-modexp-extract---extract-modulus)                       [DATA 8])
(defun (prc-modexp-extract---bbs-is-zero)                           OUTGOING_RES_LO)
(defun (prc-modexp-extract---ebs-is-zero)                           (next OUTGOING_RES_LO))
(defun (prc-modexp-extract---mbs-is-zero)                           (shift OUTGOING_RES_LO 2))
(defun (prc-modexp-extract---call-data-extends-beyond-exponent)     (shift OUTGOING_RES_LO 3))

(defconstraint prc-modexp-extract---check-bbs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-ISZERO 0 0 (prc-modexp-extract---bbs)))

(defconstraint prc-modexp-extract---check-ebs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-ISZERO 1 0 (prc-modexp-extract---ebs)))

(defconstraint prc-modexp-extract---check-mbs-is-zero (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-ISZERO 2 0 (prc-modexp-extract---mbs)))

(defconstraint prc-modexp-extract---compare-96-plus-bbs-plus-ebs-against-cds (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (call-to-LT 3 0 (+ 96 (prc-modexp-extract---bbs) (prc-modexp-extract---ebs)) 0 (prc---cds)))

(defconstraint prc-modexp-extract---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-modexp-extract---standard-precondition)))
  (begin (eq! (prc-modexp-extract---extract-modulus)
              (* (prc-modexp-extract---call-data-extends-beyond-exponent)
                 (- 1 (prc-modexp-extract---mbs-is-zero))))
         (eq! (prc-modexp-extract---extract-base)
              (* (prc-modexp-extract---extract-modulus) (- 1 (prc-modexp-extract---bbs-is-zero))))
         (eq! (prc-modexp-extract---extract-exponent)
              (* (prc-modexp-extract---extract-modulus) (- 1 (prc-modexp-extract---ebs-is-zero))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   7 Populating BLAKE2F  ;;
;;   precompiles           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 7.1 For BLAKE2F_cds   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-blake-cds---standard-precondition)   IS_BLAKE2F_CDS)
(defun (prc-blake-cds---valid-cds)  OUTGOING_RES_LO)
(defun (prc-blake-cds---r@c-is-zero)   (next OUTGOING_RES_LO))

(defconstraint prc-blake-cds---compare-cds-against-213 (:guard (* (assumption---fresh-new-stamp) (prc-blake-cds---standard-precondition)))
  (call-to-EQ 0 0 (prc---cds) 0 213))

(defconstraint prc-blake-cds---check--is-zero (:guard (* (assumption---fresh-new-stamp) (prc-blake-cds---standard-precondition)))
  (call-to-ISZERO 1 0 (prc---r@c)))

(defconstraint blake2f-a---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-blake-cds---standard-precondition)))
  (begin (eq! (prc---hub-success) (prc-blake-cds---valid-cds))
         (eq! (prc---r@c-nonzero) (- 1 (prc-blake-cds---r@c-is-zero)))))

;;;;;;;;;;;;;;;;;;;;;;;;;::;;
;;                         ;;
;; 7.2 For BLAKE2F_params  ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (prc-blake-params---standard-precondition)          IS_BLAKE2F_PARAMS)
(defun (prc-blake-params---blake-r)                        [DATA 6])
(defun (prc-blake-params---blake-f)                        [DATA 7])
(defun (prc-blake-params---sufficient-gas)                 (- 1 OUTGOING_RES_LO))
(defun (prc-blake-params---f-is-a-bit)                     (next OUTGOING_RES_LO))


(defconstraint prc-blake-params---compare-call-gas-against-blake-r (:guard (* (assumption---fresh-new-stamp) (prc-blake-params---standard-precondition)))
  (call-to-LT 0 0 (prc---callee-gas) 0 (prc-blake-params---blake-r)))

(defconstraint prc-blake-params---compare-blake-f-against-blake-f-square (:guard (* (assumption---fresh-new-stamp) (prc-blake-params---standard-precondition)))
  (call-to-EQ 1
              0
              (prc-blake-params---blake-f)
              0
              (* (prc-blake-params---blake-f) (prc-blake-params---blake-f))))

(defconstraint prc-blake-params---justify-hub-predictions (:guard (* (assumption---fresh-new-stamp) (prc-blake-params---standard-precondition)))
  (begin (eq! (prc---ram-success)
              (* (prc-blake-params---sufficient-gas) (prc-blake-params---f-is-a-bit)))
         (if-not-zero (prc---ram-success)
                      (eq! (prc---return-gas) (- (prc---callee-gas) (prc-blake-params---blake-r)))
                      (vanishes! (prc---return-gas)))))


