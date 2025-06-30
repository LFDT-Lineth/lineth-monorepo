(module mmio)

;;
;; Instruction decoding
;;
(defun (mmio_inst_weight)
  (+    (*    MMIO_INST_LIMB_VANISHES             IS_LIMB_VANISHES)
        (*    MMIO_INST_LIMB_TO_RAM_TRANSPLANT    IS_LIMB_TO_RAM_TRANSPLANT)
        (*    MMIO_INST_LIMB_TO_RAM_ONE_TARGET    IS_LIMB_TO_RAM_ONE_TARGET)
        (*    MMIO_INST_LIMB_TO_RAM_TWO_TARGET    IS_LIMB_TO_RAM_TWO_TARGET)
        (*    MMIO_INST_RAM_TO_LIMB_TRANSPLANT    IS_RAM_TO_LIMB_TRANSPLANT)
        (*    MMIO_INST_RAM_TO_LIMB_ONE_SOURCE    IS_RAM_TO_LIMB_ONE_SOURCE)
        (*    MMIO_INST_RAM_TO_LIMB_TWO_SOURCE    IS_RAM_TO_LIMB_TWO_SOURCE)
        (*    MMIO_INST_RAM_TO_RAM_TRANSPLANT     IS_RAM_TO_RAM_TRANSPLANT)
        (*    MMIO_INST_RAM_TO_RAM_PARTIAL        IS_RAM_TO_RAM_PARTIAL)
        (*    MMIO_INST_RAM_TO_RAM_TWO_TARGET     IS_RAM_TO_RAM_TWO_TARGET)
        (*    MMIO_INST_RAM_TO_RAM_TWO_SOURCE     IS_RAM_TO_RAM_TWO_SOURCE)
        (*    MMIO_INST_RAM_EXCISION              IS_RAM_EXCISION)
        (*    MMIO_INST_RAM_VANISHES              IS_RAM_VANISHES)))

(defconstraint decoding-mmio-instruction-flag ()
  (eq! MMIO_INSTRUCTION (mmio_inst_weight)))

(defun (fast-op-flag-sum)
  (force-bin (+ IS_LIMB_VANISHES
                IS_LIMB_TO_RAM_TRANSPLANT
                IS_RAM_TO_LIMB_TRANSPLANT
                IS_RAM_TO_RAM_TRANSPLANT
                IS_RAM_VANISHES)))

(defconstraint fast-op ()
  (eq! FAST (fast-op-flag-sum)))

(defun (slow-op-flag-sum)
  (force-bin (+ IS_LIMB_TO_RAM_ONE_TARGET
                IS_LIMB_TO_RAM_TWO_TARGET
                IS_RAM_TO_LIMB_ONE_SOURCE
                IS_RAM_TO_LIMB_TWO_SOURCE
                IS_RAM_TO_RAM_PARTIAL
                IS_RAM_TO_RAM_TWO_TARGET
                IS_RAM_TO_RAM_TWO_SOURCE
                IS_RAM_EXCISION)))

(defconstraint slow-op ()
  (eq! SLOW (slow-op-flag-sum)))

(defun (op-flag-sum)
  (force-bin (+ (fast-op-flag-sum) (slow-op-flag-sum))))

(defconstraint no-stamp-no-op ()
  (eq! (op-flag-sum) (~ STAMP)))

(defun (weighted-exo-sum)
  (+ (* EXO_SUM_WEIGHT_ROM EXO_IS_ROM)
     (* EXO_SUM_WEIGHT_KEC EXO_IS_KEC)
     (* EXO_SUM_WEIGHT_LOG EXO_IS_LOG)
     (* EXO_SUM_WEIGHT_TXCD EXO_IS_TXCD)
     (* EXO_SUM_WEIGHT_ECDATA EXO_IS_ECDATA)
     (* EXO_SUM_WEIGHT_RIPSHA EXO_IS_RIPSHA)
     (* EXO_SUM_WEIGHT_BLAKEMODEXP EXO_IS_BLAKEMODEXP)))

(defconstraint exo-sum-decoding ()
  (eq! (weighted-exo-sum) (* (instruction-may-provide-exo-sum) EXO_SUM)))

(defun (instruction-may-provide-exo-sum)
  (force-bin (+ IS_LIMB_VANISHES
                IS_LIMB_TO_RAM_TRANSPLANT
                IS_LIMB_TO_RAM_ONE_TARGET
                IS_LIMB_TO_RAM_TWO_TARGET
                IS_RAM_TO_LIMB_TRANSPLANT
                IS_RAM_TO_LIMB_ONE_SOURCE
                IS_RAM_TO_LIMB_TWO_SOURCE)))

;;
;; Heartbeat
;;
(defconstraint first-row (:domain {0})
  (vanishes! MMIO_STAMP))

(defconstraint stamp-increment ()
  (or! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

(defconstraint stamp-reset-ct ()
  (if-not-zero (- STAMP (prev STAMP))
               (vanishes! CT)))

(defconstraint fast-is-one-row (:guard FAST)
  (will-inc! STAMP 1))

(defconstraint slow-is-llarge-rows (:guard SLOW)
  (if-eq-else CT LLARGEMO (will-inc! STAMP 1) (will-inc! CT 1)))

(defconstraint last-row-is-finish (:domain {-1})
  (if-not-zero STAMP
               (if-eq SLOW 1 (eq! CT LLARGEMO))))

;;
;; Byte decompsition
;;
(defpurefun (byte-dec value byte acc ct)
  (begin (byte-decomposition ct acc byte)
         (if-eq ct LLARGEMO (eq! value acc))))

(defconstraint byte-decompositions ()
  (begin (byte-dec VAL_A BYTE_A ACC_A CT)
         (byte-dec VAL_B BYTE_B ACC_B CT)
         (byte-dec VAL_C BYTE_C ACC_C CT)
         (byte-dec LIMB BYTE_LIMB ACC_LIMB CT)))

;;
;; Counter constancies
;;
(defconstraint counter-constancies ()
  (begin (counter-constancy CT CN_A)
         (counter-constancy CT CN_B)
         (counter-constancy CT CN_C)
         (counter-constancy CT INDEX_A)
         (counter-constancy CT INDEX_B)
         (counter-constancy CT INDEX_C)
         (counter-constancy CT VAL_A)
         (counter-constancy CT VAL_B)
         (counter-constancy CT VAL_C)
         (counter-constancy CT VAL_A_NEW)
         (counter-constancy CT VAL_B_NEW)
         (counter-constancy CT VAL_C_NEW)
         (counter-constancy CT INDEX_X)))
