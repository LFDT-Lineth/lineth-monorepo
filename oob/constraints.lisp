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
(defun (flag-sum-inst)
  (+ IS_JUMP IS_JUMPI IS_RDC IS_CDL IS_XCALL IS_CALL IS_CREATE IS_SSTORE IS_DEPLOYMENT))

(defun (flag-sum-prc-common)
  (+ IS_ECRECOVER IS_SHA2 IS_RIPEMD IS_IDENTITY IS_ECADD IS_ECMUL IS_ECPAIRING))

(defun (flag-sum-prc-blake)
  (+ IS_BLAKE2F_CDS IS_BLAKE2F_PARAMS))

(defun (flag-sum-prc-modexp)
  (+ IS_MODEXP_CDS IS_MODEXP_XBS IS_MODEXP_LEAD IS_MODEXP_PRICING IS_MODEXP_EXTRACT))

(defun (flag-sum-prc)
  (+ (flag-sum-prc-common) (flag-sum-prc-blake) (flag-sum-prc-modexp)))

(defun (flag-sum)
  (+ (flag-sum-inst) (flag-sum-prc)))

(defun (wght-sum-inst)
  (+ (* OOB_INST_JUMP IS_JUMP)
     (* OOB_INST_JUMPI IS_JUMPI)
     (* OOB_INST_RDC IS_RDC)
     (* OOB_INST_CDL IS_CDL)
     (* OOB_INST_XCALL IS_XCALL)
     (* OOB_INST_CALL IS_CALL)
     (* OOB_INST_CREATE IS_CREATE)
     (* OOB_INST_SSTORE IS_SSTORE)
     (* OOB_INST_DEPLOYMENT IS_DEPLOYMENT)))

(defun (wght-sum-prc-common)
  (+ (* OOB_INST_ECRECOVER IS_ECRECOVER)
     (* OOB_INST_SHA2 IS_SHA2)
     (* OOB_INST_RIPEMD IS_RIPEMD)
     (* OOB_INST_IDENTITY IS_IDENTITY)
     (* OOB_INST_ECADD IS_ECADD)
     (* OOB_INST_ECMUL IS_ECMUL)
     (* OOB_INST_ECPAIRING IS_ECPAIRING)))

(defun (wght-sum-prc-blake)
  (+ (* OOB_INST_BLAKE_CDS IS_BLAKE2F_CDS) (* OOB_INST_BLAKE_PARAMS IS_BLAKE2F_PARAMS)))

(defun (wght-sum-prc-modexp)
  (+ (* OOB_INST_MODEXP_CDS IS_MODEXP_CDS)
     (* OOB_INST_MODEXP_XBS IS_MODEXP_XBS)
     (* OOB_INST_MODEXP_LEAD IS_MODEXP_LEAD)
     (* OOB_INST_MODEXP_PRICING IS_MODEXP_PRICING)
     (* OOB_INST_MODEXP_EXTRACT IS_MODEXP_EXTRACT)))

(defun (wght-sum-prc)
  (+ (wght-sum-prc-common) (wght-sum-prc-blake) (wght-sum-prc-modexp)))

(defun (wght-sum)
  (+ (wght-sum-inst) (wght-sum-prc)))

(defun (maxct-sum-inst)
  (+ (* CT_MAX_JUMP IS_JUMP)
     (* CT_MAX_JUMPI IS_JUMPI)
     (* CT_MAX_RDC IS_RDC)
     (* CT_MAX_CDL IS_CDL)
     (* CT_MAX_XCALL IS_XCALL)
     (* CT_MAX_CALL IS_CALL)
     (* CT_MAX_CREATE IS_CREATE)
     (* CT_MAX_SSTORE IS_SSTORE)
     (* CT_MAX_DEPLOYMENT IS_DEPLOYMENT)))

(defun (maxct-sum-prc-common)
  (+ (* CT_MAX_ECRECOVER IS_ECRECOVER)
     (* CT_MAX_SHA2 IS_SHA2)
     (* CT_MAX_RIPEMD IS_RIPEMD)
     (* CT_MAX_IDENTITY IS_IDENTITY)
     (* CT_MAX_ECADD IS_ECADD)
     (* CT_MAX_ECMUL IS_ECMUL)
     (* CT_MAX_ECPAIRING IS_ECPAIRING)))

(defun (maxct-sum-prc-blake)
  (+ (* CT_MAX_BLAKE2F_CDS IS_BLAKE2F_CDS) (* CT_MAX_BLAKE2F_PARAMS IS_BLAKE2F_PARAMS)))

(defun (maxct-sum-prc-modexp)
  (+ (* CT_MAX_MODEXP_CDS IS_MODEXP_CDS)
     (* CT_MAX_MODEXP_XBS IS_MODEXP_XBS)
     (* CT_MAX_MODEXP_LEAD IS_MODEXP_LEAD)
     (* CT_MAX_MODEXP_PRICING IS_MODEXP_PRICING)
     (* CT_MAX_MODEXP_EXTRACT IS_MODEXP_EXTRACT)))

(defun (maxct-sum-prc)
  (+ (maxct-sum-prc-common) (maxct-sum-prc-blake) (maxct-sum-prc-modexp)))

(defun (maxct-sum)
  (+ (maxct-sum-inst) (maxct-sum-prc)))

(defun (lookup-sum k)
  (+ (shift ADD_FLAG k) (shift MOD_FLAG k) (shift WCP_FLAG k)))

(defun (wght-lookup-sum k)
  (+ (* 1 (shift ADD_FLAG k))
     (* 2 (shift MOD_FLAG k))
     (* 3 (shift WCP_FLAG k))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                             ;;
;;    2.2 binary constraints   ;;
;;                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint binary-constraints ()
  (begin (is-binary ADD_FLAG)
         (is-binary MOD_FLAG)
         (is-binary WCP_FLAG)
         (is-binary IS_JUMP)
         (is-binary IS_JUMPI)
         (is-binary IS_RDC)
         (is-binary IS_CDL)
         (is-binary IS_XCALL)
         (is-binary IS_CALL)
         (is-binary IS_CREATE)
         (is-binary IS_SSTORE)
         (is-binary IS_DEPLOYMENT)
         (is-binary IS_ECRECOVER)
         (is-binary IS_SHA2)
         (is-binary IS_RIPEMD)
         (is-binary IS_IDENTITY)
         (is-binary IS_ECADD)
         (is-binary IS_ECMUL)
         (is-binary IS_ECPAIRING)
         (is-binary IS_BLAKE2F_CDS)
         (is-binary IS_BLAKE2F_PARAMS)
         (is-binary IS_MODEXP_CDS)
         (is-binary IS_MODEXP_XBS)
         (is-binary IS_MODEXP_LEAD)
         (is-binary IS_MODEXP_EXTRACT)
         (is-binary IS_MODEXP_PRICING)))

(defconstraint wcp-add-mod-are-exclusive ()
  (is-binary (lookup-sum 0)))

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
(defun (callToADD k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 1)
         (eq! (shift OUTGOING_INST k) EVM_INST_ADD)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (callToDIV k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 2)
         (eq! (shift OUTGOING_INST k) EVM_INST_DIV)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (callToMOD k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 2)
         (eq! (shift OUTGOING_INST k) EVM_INST_MOD)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (callToLT k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_LT)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (callToGT k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_GT)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (callToISZERO k arg_1_hi arg_1_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_ISZERO)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (debug (vanishes! (shift [OUTGOING_DATA 3] k)))
         (debug (vanishes! (shift [OUTGOING_DATA 4] k)))))

(defun (callToEQ k arg_1_hi arg_1_lo arg_2_hi arg_2_lo)
  (begin (eq! (wght-lookup-sum k) 3)
         (eq! (shift OUTGOING_INST k) EVM_INST_EQ)
         (eq! (shift [OUTGOING_DATA 1] k) arg_1_hi)
         (eq! (shift [OUTGOING_DATA 2] k) arg_1_lo)
         (eq! (shift [OUTGOING_DATA 3] k) arg_2_hi)
         (eq! (shift [OUTGOING_DATA 4] k) arg_2_lo)))

(defun (noCall k)
  (begin (eq! (wght-lookup-sum k) 0)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;  3 Populating opcodes     ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (standing-hypothesis)
  (- STAMP (prev STAMP)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.3 For JUMP       ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (jump-hypothesis)
  IS_JUMP)

(defun (jump---pc-new-hi)
  [DATA 1])

(defun (jump---pc-new-lo)
  [DATA 2])

(defun (jump---code-size)
  [DATA 5])

(defun (jump---guaranteed-exception)
  [DATA 7])

(defun (jump---jump-must-be-attempted)
  [DATA 8])

(defun (jump---valid-pc-new)
  OUTGOING_RES_LO)

(defconstraint valid-jump (:guard (* (standing-hypothesis) (jump-hypothesis)))
  (callToLT 0 (jump---pc-new-hi) (jump---pc-new-lo) 0 (jump---code-size)))

(defconstraint justify-hub-predictions-jump (:guard (* (standing-hypothesis) (jump-hypothesis)))
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
(defun (jumpi-hypothesis)
  IS_JUMPI)

(defun (jumpi---pc-new-hi)
  [DATA 1])

(defun (jumpi---pc-new-lo)
  [DATA 2])

(defun (jumpi---jump-cond-hi)
  [DATA 3])

(defun (jumpi---jump-cond-lo)
  [DATA 4])

(defun (jumpi---code-size)
  [DATA 5])

(defun (jumpi---jump-not-attempted)
  [DATA 6])

(defun (jumpi---guaranteed-exception)
  [DATA 7])

(defun (jumpi---jump-must-be-attempted)
  [DATA 8])

(defun (jumpi---valid-pc-new)
  OUTGOING_RES_LO)

(defun (jumpi---jump-cond-is-zero)
  (next OUTGOING_RES_LO))

(defconstraint valid-jumpi (:guard (* (standing-hypothesis) (jumpi-hypothesis)))
  (callToLT 0 (jumpi---pc-new-hi) (jumpi---pc-new-lo) 0 (jumpi---code-size)))

(defconstraint valid-jumpi-future (:guard (* (standing-hypothesis) (jumpi-hypothesis)))
  (callToISZERO 1 (jumpi---jump-cond-hi) (jumpi---jump-cond-lo)))

(defconstraint justify-hub-predictions-jumpi (:guard (* (standing-hypothesis) (jumpi-hypothesis)))
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
(defun (rdc-hypothesis)
  IS_RDC)

(defun (rdc---offset-hi)
  [DATA 1])

(defun (rdc---offset-lo)
  [DATA 2])

(defun (rdc---size-hi)
  [DATA 3])

(defun (rdc---size-lo)
  [DATA 4])

(defun (rdc---rds)
  [DATA 5])

(defun (rdc---rdcx)
  [DATA 7])

(defun (rdc---rdc-roob)
  (- 1 OUTGOING_RES_LO))

(defun (rdc---rdc-soob)
  (shift OUTGOING_RES_LO 2))

(defconstraint valid-rdc (:guard (* (standing-hypothesis) (rdc-hypothesis)))
  (callToISZERO 0 (rdc---offset-hi) (rdc---size-hi)))

(defconstraint valid-rdc-future (:guard (* (standing-hypothesis) (rdc-hypothesis)))
  (if-zero (rdc---rdc-roob)
           (callToADD 1 0 (rdc---offset-lo) 0 (rdc---size-lo))
           (noCall 1)))

(defconstraint valid-rdc-future-future (:guard (* (standing-hypothesis) (rdc-hypothesis)))
  (if-zero (rdc---rdc-roob)
           (begin (vanishes! (shift ADD_FLAG 2))
                  (vanishes! (shift MOD_FLAG 2))
                  (eq! (shift WCP_FLAG 2) 1)
                  (eq! (shift OUTGOING_INST 2) EVM_INST_GT)
                  (vanishes! (shift [OUTGOING_DATA 3] 2))
                  (eq! (shift [OUTGOING_DATA 4] 2) (rdc---rds)))
           (noCall 2)))

(defconstraint justify-hub-predictions-rdc (:guard (* (standing-hypothesis) (rdc-hypothesis)))
  (eq! (rdc---rdcx)
       (+ (rdc---rdc-roob)
          (* (- 1 (rdc---rdc-roob)) (rdc---rdc-soob)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.6 For               ;;
;; CALLDATALOAD          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (cdl-hypothesis)
  IS_CDL)

(defun (cdl---offset-hi)
  [DATA 1])

(defun (cdl---offset-lo)
  [DATA 2])

(defun (cdl---cds)
  [DATA 5])

(defun (cdl---cdl-out-of-bounds)
  [DATA 7])

(defun (cdl---touches-ram)
  OUTGOING_RES_LO)

(defconstraint valid-cdl (:guard (* (standing-hypothesis) (cdl-hypothesis)))
  (callToLT 0 (cdl---offset-hi) (cdl---offset-lo) 0 (cdl---cds)))

(defconstraint justify-hub-predictions-cdl (:guard (* (standing-hypothesis) (cdl-hypothesis)))
  (eq! (cdl---cdl-out-of-bounds) (- 1 (cdl---touches-ram))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.7 For               ;;
;; SSTORE                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (sstore-hypothesis)
  IS_SSTORE)

(defun (sstore---gas)
  [DATA 5])

(defun (sstore---sstorex)
  [DATA 7])

(defun (sstore---sufficient-gas)
  OUTGOING_RES_LO)

(defconstraint valid-sstore (:guard (* (standing-hypothesis) (sstore-hypothesis)))
  (callToLT 0 0 GAS_CONST_G_CALL_STIPEND 0 (sstore---gas)))

(defconstraint justify-hub-predictions-sstore (:guard (* (standing-hypothesis) (sstore-hypothesis)))
  (eq! (sstore---sstorex) (- 1 (sstore---sufficient-gas))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.8 For               ;;
;; DEPLOYMENT            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (deployment-hypothesis)
  IS_DEPLOYMENT)

(defun (deployment---code-size-hi)
  [DATA 1])

(defun (deployment---code-size-lo)
  [DATA 2])

(defun (deployment---max-code-size-exception)
  [DATA 7])

(defun (deployment---exceeds-max-code-size)
  OUTGOING_RES_LO)

(defconstraint valid-deployment (:guard (* (standing-hypothesis) (deployment-hypothesis)))
  (callToLT 0 0 24576 (deployment---code-size-hi) (deployment---code-size-lo)))

(defconstraint justify-hub-predictions-deployment (:guard (* (standing-hypothesis) (deployment-hypothesis)))
  (eq! (deployment---max-code-size-exception) (deployment---exceeds-max-code-size)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.9 For XCALL's    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (xcall-hypothesis)
  IS_XCALL)

(defun (xcall---value-hi)
  [DATA 1])

(defun (xcall---value-lo)
  [DATA 2])

(defun (xcall---value-is-nonzero)
  [DATA 7])

(defun (xcall---value-is-zero)
  [DATA 8])

(defconstraint valid-xcall (:guard (* (standing-hypothesis) (xcall-hypothesis)))
  (callToISZERO 0 (xcall---value-hi) (xcall---value-lo)))

(defconstraint justify-hub-predictions-xcall (:guard (* (standing-hypothesis) (xcall-hypothesis)))
  (begin (eq! (xcall---value-is-nonzero) (- 1 OUTGOING_RES_LO))
         (eq! (xcall---value-is-zero) OUTGOING_RES_LO)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    3.10 For CALL's    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (call-hypothesis)
  IS_CALL)

(defun (call---value-hi)
  [DATA 1])

(defun (call---value-lo)
  [DATA 2])

(defun (call---balance)
  [DATA 3])

(defun (call---call-stack-depth)
  [DATA 6])

(defun (call---value-is-nonzero)
  [DATA 7])

(defun (call---aborting-condition)
  [DATA 8])

(defun (call---insufficient-balance-abort)
  OUTGOING_RES_LO)

(defun (call---call-stack-depth-abort)
  (- 1 (next OUTGOING_RES_LO)))

(defun (call---value-is-zero)
  (shift OUTGOING_RES_LO 2))

(defconstraint valid-call (:guard (* (standing-hypothesis) (call-hypothesis)))
  (callToLT 0 0 (call---balance) (call---value-hi) (call---value-lo)))

(defconstraint valid-call-future (:guard (* (standing-hypothesis) (call-hypothesis)))
  (callToLT 1 0 (call---call-stack-depth) 0 1024))

(defconstraint valid-call-future-future (:guard (* (standing-hypothesis) (call-hypothesis)))
  (callToISZERO 2 (call---value-hi) (call---value-lo)))

(defconstraint justify-hub-predictions-call (:guard (* (standing-hypothesis) (call-hypothesis)))
  (begin (eq! (call---value-is-nonzero) (- 1 (call---value-is-zero)))
         (eq! (call---aborting-condition)
              (+ (call---insufficient-balance-abort)
                 (* (- 1 (call---insufficient-balance-abort)) (call---call-stack-depth-abort))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 3.11 For              ;;
;; CREATE's              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (create-hypothesis)
  IS_CREATE)

(defun (create---value-hi)
  [DATA 1])

(defun (create---value-lo)
  [DATA 2])

(defun (create---balance)
  [DATA 3])

(defun (create---nonce)
  [DATA 4])

(defun (create---has-code)
  [DATA 5])

(defun (create---call-stack-depth)
  [DATA 6])

(defun (create---aborting-condition)
  [DATA 7])

(defun (create---failure-condition)
  [DATA 8])

(defun (create---creator-nonce)
  [DATA 9])

(defun (create---insufficient-balance-abort)
  OUTGOING_RES_LO)

(defun (create---stack-depth-abort)
  (- 1 (next OUTGOING_RES_LO)))

(defun (create---nonzero-nonce)
  (- 1 (shift OUTGOING_RES_LO 2)))

(defun (create---creator-nonce-abort)
  (- 1 (shift OUTGOING_RES_LO 3)))

(defun (create---aborting-conditions-sum)
  (+ (create---insufficient-balance-abort) (create---stack-depth-abort) (create---creator-nonce-abort)))

(defconstraint valid-create (:guard (* (standing-hypothesis) (create-hypothesis)))
  (callToLT 0 0 (create---balance) (create---value-hi) (create---value-lo)))

(defconstraint valid-create-future (:guard (* (standing-hypothesis) (create-hypothesis)))
  (callToLT 1 0 (create---call-stack-depth) 0 1024))

(defconstraint valid-create-future-future (:guard (* (standing-hypothesis) (create-hypothesis)))
  (callToISZERO 2 0 (create---nonce)))

(defconstraint valid-create-future-future-future (:guard (* (standing-hypothesis) (create-hypothesis)))
  (callToLT 3 0 (create---creator-nonce) 0 18446744073709551615)) ; (create---creator-nonce) < 2^64 - 1

(defconstraint justify-hub-predictions-create (:guard (* (standing-hypothesis) (create-hypothesis)))
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
(defun (prc-hypothesis)
  (flag-sum-prc))

(defun (prc-common-hypothesis)
  (flag-sum-prc-common))

(defun (prc---call-gas)
  [DATA 1])

(defun (prc---cds)
  [DATA 2])

(defun (prc---r-at-c)
  [DATA 3])

(defun (prc---hub-success)
  [DATA 4])

(defun (prc---ram-success)
  [DATA 4])

(defun (prc---return-gas)
  [DATA 5])

(defun (prc---extract-call-data)
  [DATA 6])

(defun (prc---empty-call-data)
  [DATA 7])

(defun (prc---r-at-c-nonzero)
  [DATA 8])

;;
(defun (prc---cds-is-zero)
  OUTGOING_RES_LO)

(defun (prc---r-at-c-is-zero)
  (next OUTGOING_RES_LO))

(defconstraint valid-prc (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-common-hypothesis)))
  (callToISZERO 0 0 (prc---cds)))

(defconstraint valid-prc-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-common-hypothesis)))
  (callToISZERO 1 0 (prc---r-at-c)))

(defconstraint justify-hub-predictions-prc (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-common-hypothesis)))
  (begin (eq! (prc---extract-call-data)
              (* (prc---hub-success) (- 1 (prc---cds-is-zero))))
         (eq! (prc---empty-call-data) (* (prc---hub-success) (prc---cds-is-zero)))
         (eq! (prc---r-at-c-nonzero) (- 1 (prc---r-at-c-is-zero)))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 5.2 For ECRECOVER,    ;;
;; ECADD, ECMUL          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (prc-ecrecover-prc-ecadd-prc-ecmul-hypothesis)
  (+ IS_ECRECOVER IS_ECADD IS_ECMUL))

(defun (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost)
  (+ (* 3000 IS_ECRECOVER) (* 150 IS_ECADD) (* 6000 IS_ECMUL)))

(defun (prc-ecrecover-prc-ecadd-prc-ecmul---insufficient-gas)
  (shift OUTGOING_RES_LO 2))

(defconstraint valid-prc-ecrecover-prc-ecadd-prc-ecmul-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-ecrecover-prc-ecadd-prc-ecmul-hypothesis)))
  (callToLT 2 0 (prc---call-gas) 0 (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost)))

(defconstraint justify-hub-predictions-prc-ecrecover-prc-ecadd-prc-ecmul (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-ecrecover-prc-ecadd-prc-ecmul-hypothesis)))
  (begin (eq! (prc---hub-success) (- 1 (prc-ecrecover-prc-ecadd-prc-ecmul---insufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---call-gas) (prc-ecrecover-prc-ecadd-prc-ecmul---precompile-cost))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 5.3 For SHA2-256,     ;;
;; RIPEMD-160, IDENTITY  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (prc-sha2-prc-ripemd-prc-identity-hypothesis)
  (+ IS_SHA2 IS_RIPEMD IS_IDENTITY))

(defun (prc-sha2-prc-ripemd-prc-identity---ceil)
  (shift OUTGOING_RES_LO 2))

(defun (prc-sha2-prc-ripemd-prc-identity---insufficient-gas)
  (shift OUTGOING_RES_LO 3))

(defun (prc-sha2-prc-ripemd-prc-identity---precompile-cost)
  (* (+ 5 (prc-sha2-prc-ripemd-prc-identity---ceil))
     (+ (* 12 IS_SHA2) (* 120 IS_RIPEMD) (* 3 IS_IDENTITY))))

(defconstraint valid-prc-sha2-prc-ripemd-prc-identity-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-sha2-prc-ripemd-prc-identity-hypothesis)))
  (callToDIV 2 0 (+ (prc---cds) 31) 0 32))

(defconstraint valid-prc-sha2-prc-ripemd-prc-identity-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-sha2-prc-ripemd-prc-identity-hypothesis)))
  (callToLT 3 0 (prc---call-gas) 0 (prc-sha2-prc-ripemd-prc-identity---precompile-cost)))

(defconstraint justify-hub-predictions-prc-sha2-prc-ripemd-prc-identity (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-sha2-prc-ripemd-prc-identity-hypothesis)))
  (begin (eq! (prc---hub-success) (- 1 (prc-sha2-prc-ripemd-prc-identity---insufficient-gas)))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas)
                       (- (prc---call-gas) (prc-sha2-prc-ripemd-prc-identity---precompile-cost))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;; 4.4 For ECPAIRING     ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (prc-ecpairing-hypothesis)
  IS_ECPAIRING)

(defun (prc-ecpairing---remainder)
  (shift OUTGOING_RES_LO 2))

(defun (prc-ecpairing---is-multiple_192)
  (shift OUTGOING_RES_LO 3))

(defun (prc-ecpairing---insufficient-gas)
  (shift OUTGOING_RES_LO 4))

(defun (prc-ecpairing---precompile-cost192)
  (* (prc-ecpairing---is-multiple_192)
     (+ (* 45000 192) (* 34000 (prc---cds)))))

(defconstraint valid-prc-ecpairing-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-ecpairing-hypothesis)))
  (callToMOD 2 0 (prc---cds) 0 192))

(defconstraint valid-prc-ecpairing-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-ecpairing-hypothesis)))
  (callToISZERO 3 0 (prc-ecpairing---remainder)))

(defconstraint valid-prc-ecpairing-future-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-ecpairing-hypothesis)))
  (if-zero (prc-ecpairing---is-multiple_192)
           (noCall 4)
           (begin (vanishes! (shift ADD_FLAG 4))
                  (vanishes! (shift MOD_FLAG 4))
                  (eq! (shift WCP_FLAG 4) 1)
                  (eq! (shift OUTGOING_INST 4) EVM_INST_LT)
                  (vanishes! (shift [OUTGOING_DATA 1] 4))
                  (eq! (shift [OUTGOING_DATA 2] 4) (prc---call-gas))
                  (vanishes! (shift [OUTGOING_DATA 3] 4))
                  (eq! (* (shift [OUTGOING_DATA 4] 4) 192)
                       (prc-ecpairing---precompile-cost192)))))

(defconstraint justify-hub-predictions-prc-ecpairing (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-ecpairing-hypothesis)))
  (begin (eq! (prc---hub-success)
              (* (prc-ecpairing---is-multiple_192) (- 1 (prc-ecpairing---insufficient-gas))))
         (if-zero (prc---hub-success)
                  (vanishes! (prc---return-gas))
                  (eq! (* (prc---return-gas) 192)
                       (- (* (prc---call-gas) 192) (prc-ecpairing---precompile-cost192))))))

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
(defun (prc-modexp-cds-hypothesis)
  IS_MODEXP_CDS)

(defun (prc-modexp-cds---extract-bbs)
  [DATA 3])

(defun (prc-modexp-cds---extract-ebs)
  [DATA 4])

(defun (prc-modexp-cds---extract-mbs)
  [DATA 5])

(defconstraint valid-prc-modexp-cds (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-cds-hypothesis)))
  (callToLT 0 0 0 0 (prc---cds)))

(defconstraint valid-prc-modexp-cds-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-cds-hypothesis)))
  (callToLT 1 0 32 0 (prc---cds)))

(defconstraint valid-prc-modexp-cds-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-cds-hypothesis)))
  (callToLT 2 0 64 0 (prc---cds)))

(defconstraint justify-hub-predictions-prc-modexp-cds (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-cds-hypothesis)))
  (begin (eq! (prc-modexp-cds---extract-bbs) OUTGOING_RES_LO)
         (eq! (prc-modexp-cds---extract-ebs) (next OUTGOING_RES_LO))
         (eq! (prc-modexp-cds---extract-mbs) (shift OUTGOING_RES_LO 2))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.2 For MODEXP - xbs  ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (prc-modexp-xbs-hypothesis)
  IS_MODEXP_XBS)

(defun (prc-modexp-xbs---xbs-hi)
  [DATA 1])

(defun (prc-modexp-xbs---xbs-lo)
  [DATA 2])

(defun (prc-modexp-xbs---ybs-lo)
  [DATA 3])

(defun (prc-modexp-xbs---compute-max)
  [DATA 4])

(defun (prc-modexp-xbs---max-xbs-ybs)
  [DATA 7])

(defun (prc-modexp-xbs---xbs-nonzero)
  [DATA 8])

(defun (prc-modexp-xbs---compo-to_512)
  OUTGOING_RES_LO)

(defun (prc-modexp-xbs---comp)
  (next OUTGOING_RES_LO))

(defconstraint valid-prc-modexp-xbs (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-xbs-hypothesis)))
  (callToLT 0 (prc-modexp-xbs---xbs-hi) (prc-modexp-xbs---xbs-lo) 0 513))

(defconstraint valid-prc-modexp-xbs-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-xbs-hypothesis)))
  (callToLT 1 0 (prc-modexp-xbs---xbs-lo) 0 (prc-modexp-xbs---ybs-lo)))

(defconstraint valid-prc-modexp-xbs-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-xbs-hypothesis)))
  (callToISZERO 2 0 (prc-modexp-xbs---xbs-lo)))

(defconstraint additional-prc-modexp-xbs (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-xbs-hypothesis)))
  (begin (vanishes! (* (prc-modexp-xbs---compute-max) (- 1 (prc-modexp-xbs---compute-max))))
         (eq! (prc-modexp-xbs---compo-to_512) 1)))

(defconstraint justify-hub-predictions-prc-modexp-xbs (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-xbs-hypothesis)))
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
(defun (prc-modexp-lead-hypothesis)
  IS_MODEXP_LEAD)

(defun (prc-modexp-lead---bbs)
  [DATA 1])

(defun (prc-modexp-lead---ebs)
  [DATA 3])

(defun (prc-modexp-lead---load-lead)
  [DATA 4])

(defun (prc-modexp-lead---cds-cutoff)
  [DATA 6])

(defun (prc-modexp-lead---ebs-cutoff)
  [DATA 7])

(defun (prc-modexp-lead---sub-ebs_32)
  [DATA 8])

(defun (prc-modexp-lead---ebs-is-zero)
  OUTGOING_RES_LO)

(defun (prc-modexp-lead---ebs-less-than_32)
  (next OUTGOING_RES_LO))

(defun (prc-modexp-lead---call-data-contains-exponent-bytes)
  (shift OUTGOING_RES_LO 2))

(defun (prc-modexp-lead---comp)
  (shift OUTGOING_RES_LO 3))

(defconstraint valid-prc-modexp-lead (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-lead-hypothesis)))
  (callToISZERO 0 0 (prc-modexp-lead---ebs)))

(defconstraint valid-prc-modexp-lead-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-lead-hypothesis)))
  (callToLT 1 0 (prc-modexp-lead---ebs) 0 32))

(defconstraint valid-prc-modexp-lead-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-lead-hypothesis)))
  (callToLT 2 0 (+ 96 (prc-modexp-lead---ebs)) 0 (prc---cds)))

(defconstraint valid-prc-modexp-lead-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-lead-hypothesis)))
  (callToLT 3
            0
            (- (prc---cds) (+ 96 (prc-modexp-lead---ebs)))
            0
            32))

(defconstraint justify-hub-predictions-prc-modexp-lead (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-lead-hypothesis)))
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
(defun (prc-modexp-pricing-hypothesis)
  IS_MODEXP_PRICING)

(defun (prc-modexp-pricing---exponent-log)
  [DATA 6])

(defun (prc-modexp-pricing---max-xbs-ybs)
  [DATA 7])

(defun (prc-modexp-pricing---exponent-log-is-zero)
  (next OUTGOING_RES_LO))

(defun (prc-modexp-pricing---f-of-max)
  (shift OUTGOING_RES_LO 2))

(defun (prc-modexp-pricing---big-quotient)
  (shift OUTGOING_RES_LO 3))

(defun (prc-modexp-pricing---big-quotient_LT_200)
  (shift OUTGOING_RES_LO 4))

(defun (prc-modexp-pricing---big-numerator)
  (if-zero (prc-modexp-pricing---exponent-log-is-zero)
           (* (prc-modexp-pricing---f-of-max) (prc-modexp-pricing---exponent-log))
           (prc-modexp-pricing---f-of-max)))

(defun (prc-modexp-pricing---precompile-cost)
  (if-zero (prc-modexp-pricing---big-quotient_LT_200)
           (prc-modexp-pricing---big-quotient)
           200))

(defconstraint valid-prc-modexp-pricing (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (callToISZERO 0 0 (prc---r-at-c)))

(defconstraint valid-prc-modexp-pricing-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (callToISZERO 1 0 (prc-modexp-pricing---exponent-log)))

(defconstraint valid-prc-modexp-pricing-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (callToDIV 2
             0
             (+ (* (prc-modexp-pricing---max-xbs-ybs) (prc-modexp-pricing---max-xbs-ybs)) 7)
             0
             8))

(defconstraint valid-prc-modexp-pricing-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (callToDIV 3 0 (prc-modexp-pricing---big-numerator) 0 G_QUADDIVISOR))

(defconstraint valid-prc-modexp-pricing-future-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (callToLT 4 0 (prc-modexp-pricing---big-quotient) 0 200))

(defconstraint valid-prc-modexp-pricing-future-future-future-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (callToLT 5 0 (prc---call-gas) 0 (prc-modexp-pricing---precompile-cost)))

(defconstraint justify-hub-predictions-prc-modexp-pricing (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-pricing-hypothesis)))
  (begin (eq! (prc---ram-success)
              (- 1 (shift OUTGOING_RES_LO 5)))
         (if-zero (prc---ram-success)
                  (vanishes! (prc---return-gas))
                  (eq! (prc---return-gas) (- (prc---call-gas) (prc-modexp-pricing---precompile-cost))))
         (eq! (prc---r-at-c-nonzero) (- 1 OUTGOING_RES_LO))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;   6.5 For MODEXP        ;;
;;   - extract             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (prc-modexp-extract-hypothesis)
  IS_MODEXP_EXTRACT)

(defun (prc-modexp-extract---bbs)
  [DATA 3])

(defun (prc-modexp-extract---ebs)
  [DATA 4])

(defun (prc-modexp-extract---mbs)
  [DATA 5])

(defun (prc-modexp-extract---extract-base)
  [DATA 6])

(defun (prc-modexp-extract---extract-exponent)
  [DATA 7])

(defun (prc-modexp-extract---extract-modulus)
  [DATA 8])

(defun (prc-modexp-extract---bbs-is-zero)
  OUTGOING_RES_LO)

(defun (prc-modexp-extract---ebs-is-zero)
  (next OUTGOING_RES_LO))

(defun (prc-modexp-extract---mbs-is-zero)
  (shift OUTGOING_RES_LO 2))

(defun (prc-modexp-extract---call-data-extends-beyond-exponent)
  (shift OUTGOING_RES_LO 3))

(defconstraint valid-prc-modexp-extract (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-extract-hypothesis)))
  (callToISZERO 0 0 (prc-modexp-extract---bbs)))

(defconstraint valid-prc-modexp-extract-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-extract-hypothesis)))
  (callToISZERO 1 0 (prc-modexp-extract---ebs)))

(defconstraint valid-prc-modexp-extract-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-extract-hypothesis)))
  (callToISZERO 2 0 (prc-modexp-extract---mbs)))

(defconstraint justify-hub-predictions-prc-modexp-extract (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-modexp-extract-hypothesis)))
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
(defun (prc-blake-cds-hypothesis)
  IS_BLAKE2F_CDS)

(defun (prc-blake-cds---valid-cds)
  OUTGOING_RES_LO)

(defun (prc-blake-cds---r-at-c-is-zero)
  (next OUTGOING_RES_LO))

(defconstraint valid-prc-blake-cds (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-cds-hypothesis)))
  (callToEQ 0 0 (prc---cds) 0 213))

(defconstraint valid-prc-blake-cds-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-cds-hypothesis)))
  (callToISZERO 1 0 (prc---r-at-c)))

(defconstraint justify-hub-predictions-blake2f-a (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-cds-hypothesis)))
  (begin (eq! (prc---hub-success) (prc-blake-cds---valid-cds))
         (eq! (prc---r-at-c-nonzero) (- 1 (prc-blake-cds---r-at-c-is-zero)))))

;;;;;;;;;;;;;;;;;;;;;;;;;::;;
;;                         ;;
;; 7.2 For BLAKE2F_params  ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (prc-blake-params-hypothesis)
  IS_BLAKE2F_PARAMS)

(defun (prc-blake-params---blake-r)
  [DATA 6])

(defun (prc-blake-params---blake-f)
  [DATA 7])

(defun (prc-blake-params---sufficient-gas)
  (- 1 OUTGOING_RES_LO))

(defun (prc-blake-params---f-is-a-bit)
  (next OUTGOING_RES_LO))

(defconstraint valid-prc-blake-params (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-params-hypothesis)))
  (callToLT 0 0 (prc---call-gas) 0 (prc-blake-params---blake-r)))

(defconstraint valid-prc-blake-params-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-params-hypothesis)))
  (callToEQ 1
            0
            (prc-blake-params---blake-f)
            0
            (* (prc-blake-params---blake-f) (prc-blake-params---blake-f))))

(defconstraint valid-prc-blake-params-future-future (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-params-hypothesis)))
  (callToISZERO 2 0 (prc---r-at-c)))

(defconstraint justify-hub-predictions-prc-blake-params (:guard (* (standing-hypothesis) (prc-hypothesis) (prc-blake-params-hypothesis)))
  (begin (eq! (prc---ram-success)
              (* (prc-blake-params---sufficient-gas) (prc-blake-params---f-is-a-bit)))
         (if-not-zero (prc---ram-success)
                      (eq! (prc---return-gas) (- (prc---call-gas) (prc-blake-params---blake-f)))
                      (vanishes! (prc---return-gas)))))


