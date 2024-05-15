(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   9.5 MISC/STP constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (set-STP-instruction-create   rel_offset
                                      instruction
                                      value_hi
                                      value_lo
                                      mxp_gas)
  (begin (eq!  (shift  misc/STP_INSTRUCTION  rel_offset)  instruction)
         (eq!  (shift  misc/STP_VALUE_HI     rel_offset)  value_hi)
         (eq!  (shift  misc/STP_VALUE_LO     rel_offset)  value_lo)
         (eq!  (shift  misc/STP_GAS_MXP      rel_offset)  mxp_gas)))


(defun  (set-STP-instruction-call   rel_offset
                                    instruction
                                    gas_hi
                                    gas_lo
                                    value_hi
                                    value_lo
                                    target_exists
                                    target_warmth
                                    mxp_gas)
  (begin (eq!  (shift  misc/STP_INSTRUCTION  rel_offset)  instruction)
         (eq!  (shift  misc/STP_GAS_HI       rel_offset)  gas_hi)
         (eq!  (shift  misc/STP_GAS_LO       rel_offset)  gas_lo)
         (eq!  (shift  misc/STP_VALUE_HI     rel_offset)  value_hi)
         (eq!  (shift  misc/STP_VALUE_LO     rel_offset)  value_lo)
         (eq!  (shift  misc/STP_EXISTS       rel_offset)  target_exists)
         (eq!  (shift  misc/STP_WARMTH       rel_offset)  target_warmth)
         (eq!  (shift  misc/STP_GAS_MXP      rel_offset)  mxp_gas)))
