(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   9.4 MISC/OOB constraints: opcodes   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (set-OOB-instruction-jump
         kappa               ;; offset
         pc_new_hi           ;; high part of proposed new program counter
         pc_new_lo           ;; low  part of proposed new program counter
         code_size           ;; code size of byte code currently executing
         ) (begin
         (eq! (shift misc/OOB_INST             kappa) OOB_INST_JUMP )
         (eq! (shift [ misc/OOB_DATA 1 ]       kappa) pc_new_hi)
         (eq! (shift [ misc/OOB_DATA 2 ]       kappa) pc_new_lo)
         ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 5 ]       kappa) code_size)
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))

(defun (set-OOB-instruction-jumpi
         kappa               ;; offset
         pc_new_hi           ;; high part of proposed new program counter
         pc_new_lo           ;; low  part of proposed new program counter
         jump_condition_hi   ;; high part of jump condition
         jump_condition_lo   ;; low  part of jump condition
         code_size           ;; code size of byte code currently executing
         ) (begin
         (eq! (shift misc/OOB_INST             kappa) OOB_INST_JUMPI)
         (eq! (shift [ misc/OOB_DATA 1 ]       kappa) pc_new_hi)
         (eq! (shift [ misc/OOB_DATA 2 ]       kappa) pc_new_lo)
         (eq! (shift [ misc/OOB_DATA 3 ]       kappa) jump_condition_hi)
         (eq! (shift [ misc/OOB_DATA 4 ]       kappa) jump_condition_lo)
         (eq! (shift [ misc/OOB_DATA 5 ]       kappa) code_size)
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))

(defun (set-OOB-instruction-sstore
         kappa               ;; offset
         gas_actual          ;; GAS_ACTUAL
         ) (begin
         (eq! (shift misc/OOB_INST          kappa) OOB_INST_SSTORE )
         ;; (eq! (shift [ misc/OOB_DATA 1 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 2 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 5 ]    kappa) gas_actual)
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))

(defun (set-OOB-instruction-cdl
         kappa               ;; row offset
         offset_hi           ;; offset within call data, high part
         offset_lo           ;; offset within call data, low  part
         call_data_size      ;; call data size
         ) (begin
         (eq! (shift misc/OOB_INST          kappa) OOB_INST_CDL )
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa) offset_hi)
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa) offset_lo)
         ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 5 ]    kappa) call_data_size)
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))

(defun (set-OOB-instruction-rdc
         kappa                   ;; row offset
         source_offset_hi        ;; offset within call data, high part
         source_offset_lo        ;; offset within call data, low  part
         size_hi                 ;; size of data to copy, high part
         size_lo                 ;; size of data to copy, low  part
         return_data_size        ;; return data size
         ) (begin
         (eq! (shift misc/OOB_INST          kappa) OOB_INST_RDC)
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa) source_offset_hi)
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa) source_offset_lo)
         (eq! (shift [ misc/OOB_DATA 3 ]    kappa) size_hi)
         (eq! (shift [ misc/OOB_DATA 4 ]    kappa) size_lo)
         (eq! (shift [ misc/OOB_DATA 5 ]    kappa) return_data_size)
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))

(defun (set-OOB-instruction-deployment
         kappa                            ;; offset
         code_size_hi                     ;; code size hi
         code_size_lo                     ;; code size lo
         ) (begin
         (eq! (shift misc/OOB_INST          kappa)   OOB_INST_DEPLOYMENT )
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa)   code_size_hi)
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa)   code_size_lo)
         ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )    ;; max code size exception
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-xcall
         kappa           ;; offset
         value_hi        ;; value (high part)
         value_lo        ;; value (low  part, stack argument of CALL-type instruction)
         ) (begin
         (eq!    (shift misc/OOB_INST          kappa)   OOB_INST_XCALL )
         (eq!    (shift [ misc/OOB_DATA 1 ]    kappa)   value_hi       )
         (eq!    (shift [ misc/OOB_DATA 2 ]    kappa)   value_lo       )
         ;; (eq!    (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq!    (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq!    (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq!    (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq!    (shift [ misc/OOB_DATA 7 ]    kappa) )    ;; value_is_nonzero
         ;; (eq!    (shift [ misc/OOB_DATA 8 ]    kappa) )    ;; value_is_zero    ... I don't remember why I ask for both ...
         ))


(defun (set-OOB-instruction-call
         kappa              ;; offset
         value_hi           ;; value   (high part)
         value_lo           ;; value   (low  part, stack argument of CALL-type instruction)
         balance            ;; balance (from caller account)
         call_stack_depth   ;; call stack depth
         ) (begin
         (eq!    (shift misc/OOB_INST          kappa)   OOB_INST_CALL   )
         (eq!    (shift [ misc/OOB_DATA 1 ]    kappa)   value_hi        )
         (eq!    (shift [ misc/OOB_DATA 2 ]    kappa)   value_lo        )
         (eq!    (shift [ misc/OOB_DATA 3 ]    kappa)   balance         )
         ;; (eq!    (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq!    (shift [ misc/OOB_DATA 5 ]    kappa) )
         (eq!    (shift [ misc/OOB_DATA 6 ]    kappa)   call_stack_depth)
         ;; (eq!    (shift [ misc/OOB_DATA 7 ]    kappa) )    ;; value_is_nonzero
         ;; (eq!    (shift [ misc/OOB_DATA 8 ]    kappa) )    ;; aborting condition
         ))


(defun (set-OOB-instruction-create
         kappa              ;; offset
         value_hi           ;; value   (high part)
         value_lo           ;; value   (low  part, stack argument of CALL-type instruction)
         balance            ;; balance (from caller account)
         nonce              ;; callee's nonce
         has_code           ;; callee's HAS_CODE
         call_stack_depth   ;; current call stack depth
         ) (begin
         (eq!    (shift misc/OOB_INST          kappa)   OOB_INST_CREATE  )
         (eq!    (shift [ misc/OOB_DATA 1 ]    kappa)   value_hi         )
         (eq!    (shift [ misc/OOB_DATA 2 ]    kappa)   value_lo         )
         (eq!    (shift [ misc/OOB_DATA 3 ]    kappa)   balance          )
         (eq!    (shift [ misc/OOB_DATA 4 ]    kappa)   nonce            )
         (eq!    (shift [ misc/OOB_DATA 5 ]    kappa)   has_code         )
         (eq!    (shift [ misc/OOB_DATA 6 ]    kappa)   call_stack_depth )
         ;; (eq!    (shift [ misc/OOB_DATA 7 ]    kappa) )    ;; value_is_nonzero
         ;; (eq!    (shift [ misc/OOB_DATA 8 ]    kappa) )    ;; aborting condition
         ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;   9.4 MISC/OOB constraints: precompiles   ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (set-OOB-instruction-common
         kappa                            ;; offset
         common_precompile_oob_inst       ;; relevant OOB instruction
         call_gas                         ;; call gas i.e. gas provided to the precompile
         cds                              ;; call data size
         r@c                              ;; return at capacity, final argument of any CALL
         ) (begin
         (eq!    (shift   misc/OOB_INST        kappa)    common_precompile_oob_inst )
         (eq!    (shift [ misc/OOB_DATA 1 ]    kappa)    call_gas                   )
         (eq!    (shift [ misc/OOB_DATA 2 ]    kappa)    cds                        )
         (eq!    (shift [ misc/OOB_DATA 3 ]    kappa)    r@c                        )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-modexp-cds
         kappa                            ;; offset
         cds                              ;; call data size
         ) (begin
         (eq! (shift misc/OOB_INST            kappa) OOB_INST_MODEXP_CDS )
         ;; (eq! (shift [ misc/OOB_DATA 1 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa) cds )
         ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-modexp-xbs
         kappa                            ;; offset
         xbs_hi                           ;; high part of some {b,e,m}bs
         xbs_lo                           ;; low  part of some {b,e,m}bs
         ybs_lo                           ;; low  part of some {b,e,m}bs
         compute_max                      ;; bit indicating whether to compute max(xbs, ybs) or not
         ) (begin
         (eq! (shift misc/OOB_INST            kappa) OOB_INST_MODEXP_XBS )
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa) xbs_hi      )
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa) xbs_lo      )
         (eq! (shift [ misc/OOB_DATA 3 ]    kappa) ybs_lo      )
         (eq! (shift [ misc/OOB_DATA 4 ]    kappa) compute_max )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-modexp-lead
         kappa                            ;; offset
         bbs_lo                           ;; low part of bbs (base     byte size)
         cds                              ;; call data size
         ebs_lo                           ;; low part of ebs (exponent byte size)
         ) (begin
         (eq! (shift misc/OOB_INST            kappa) OOB_INST_MODEXP_LEAD )
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa) bbs_lo )
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa) cds    )
         (eq! (shift [ misc/OOB_DATA 3 ]    kappa) ebs_lo )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-modexp-pricing
         kappa                            ;; offset
         call_gas                         ;; call gas i.e. gas provided to the precompile
         r@c                              ;; return at capacity, final argument of any CALL
         exponent_log                     ;; leading (≤) word log of exponent
         max_mbs_bbs                      ;; call data size
         ) (begin
         (eq! (shift misc/OOB_INST            kappa) OOB_INST_MODEXP_PRICING )
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa) call_gas )
         ;; (eq! (shift [ misc/OOB_DATA 2 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 3 ]    kappa) r@c )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 6 ]    kappa) exponent_log )
         (eq! (shift [ misc/OOB_DATA 7 ]    kappa) max_mbs_bbs )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-modexp-extract
         kappa                            ;; offset
         cds                              ;; call data size
         bbs_lo                           ;; low part of bbs (base     byte size)
         ebs_lo                           ;; low part of ebs (exponent byte size)
         mbs_lo                           ;; low part of mbs (modulus  byte size)
         ) (begin
         (eq!    (shift   misc/OOB_INST        kappa) OOB_INST_MODEXP_EXTRACT )
         ;; (eq! (shift [ misc/OOB_DATA 1 ]    kappa) )
         (eq!    (shift [ misc/OOB_DATA 2 ]    kappa) cds    )
         (eq!    (shift [ misc/OOB_DATA 3 ]    kappa) bbs_lo )
         (eq!    (shift [ misc/OOB_DATA 4 ]    kappa) ebs_lo )
         (eq!    (shift [ misc/OOB_DATA 5 ]    kappa) mbs_lo )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-blake-cds
         kappa                            ;; offset
         cds                              ;; call data size
         r@c                              ;; return at capacity, final argument of any CALL
         ) (begin
         (eq! (shift misc/OOB_INST            kappa) OOB_INST_blakeCds )
         ;; (eq! (shift [ misc/OOB_DATA 1 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 2 ]    kappa) cds )
         (eq! (shift [ misc/OOB_DATA 3 ]    kappa) r@c )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))


(defun (set-OOB-instruction-blake
         kappa                            ;; offset
         call_gas                         ;; call gas i.e. gas provided to the precompile
         blake_r                          ;; rounds parameter of the call data of BLAKE2f
         blake_f                          ;; f      parameter of the call data of BLAKE2f ("final block indicator")
         ) (begin
         (eq! (shift misc/OOB_INST            kappa) OOB_INST_blakeParams )
         (eq! (shift [ misc/OOB_DATA 1 ]    kappa) call_gas )
         ;; (eq! (shift [ misc/OOB_DATA 2 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
         ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
         (eq! (shift [ misc/OOB_DATA 6 ]    kappa) blake_r )
         (eq! (shift [ misc/OOB_DATA 7 ]    kappa) blake_f )
         ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
         ))






;; (defun (set-OOB-instruction-Z
;;          kappa                            ;; offset
;;          ) (begin
;;          (eq! (shift misc/OOB_INST            kappa) OOB_INST_ )
;;          ;; (eq! (shift [ misc/OOB_DATA 1 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 2 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 3 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 4 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 5 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 6 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 7 ]    kappa) )
;;          ;; (eq! (shift [ misc/OOB_DATA 8 ]    kappa) )
;;          ))
