(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   9.5 MISC/STP constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun  (set-STP-instruction-create   rel_offset      ;; relative row offset
                                      instruction     ;; instruction
                                      value_hi        ;; value to transfer, high part
                                      value_lo        ;; value to transfer, low  part
                                      mxp_gas         ;; memory expansion gas
                                      )
  (begin (eq!  (shift  misc/STP_INSTRUCTION  rel_offset)  instruction)
         (eq!  (shift  misc/STP_VALUE_HI     rel_offset)  value_hi)
         (eq!  (shift  misc/STP_VALUE_LO     rel_offset)  value_lo)
         (eq!  (shift  misc/STP_GAS_MXP      rel_offset)  mxp_gas)))


(defun  (set-STP-instruction-call   rel_offset               ;; relative row offset
                                    instruction              ;; instruction
                                    gas_hi                   ;; max gas allowance argument, high part
                                    gas_lo                   ;; max gas allowance argument, low  part
                                    value_hi                 ;; value to transfer, high part
                                    value_lo                 ;; value to transfer, low  part
                                    target_exists            ;; bit indicating target account existence
                                    target_warmth            ;; bit indicating target account warmth
                                    is_delegated             ;; bit indicating whether target account is delegated
                                    is_delegated_to_self     ;; bit indicating whether target account is delegated to self
                                    delegate_warmth          ;; bit indicating the warmth of the delegate
                                    mxp_gas                  ;; memory expansion gas
                                    )
  (begin
         (eq!   (shift misc/STP_INSTRUCTION                   rel_offset)   instruction           )
         (eq!   (shift misc/STP_GAS_HI                        rel_offset)   gas_hi                )
         (eq!   (shift misc/STP_GAS_LO                        rel_offset)   gas_lo                )
         (eq!   (shift misc/STP_VALUE_HI                      rel_offset)   value_hi              )
         (eq!   (shift misc/STP_VALUE_LO                      rel_offset)   value_lo              )
         (eq!   (shift misc/STP_EXISTS                        rel_offset)   target_exists         )
         (eq!   (shift misc/STP_WARMTH                        rel_offset)   target_warmth         )
         (eq!   (shift misc/STP_CALLEE_IS_DELEGATED           rel_offset)   is_delegated          )
         (eq!   (shift misc/STP_CALLEE_IS_DELEGATED_TO_SELF   rel_offset)   is_delegated_to_self  )
         (eq!   (shift misc/STP_DELEGATE_WARMTH               rel_offset)   delegate_warmth       )
         (eq!   (shift misc/STP_GAS_MXP                       rel_offset)   mxp_gas               )))
