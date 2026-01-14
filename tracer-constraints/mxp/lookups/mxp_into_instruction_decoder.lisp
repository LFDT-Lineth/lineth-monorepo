(defun (mxp-into-instruction-decoder-selector) mxp.DECODER) ;; ""

(defclookup mxp-into-instdecoder
  ;; target columns
  (
   instdecoder.MXP_FLAG
   instdecoder.OPCODE
   instdecoder.IS_MSIZE
   instdecoder.IS_RETURN
   instdecoder.IS_MCOPY
   instdecoder.IS_FIXED_SIZE_32
   instdecoder.IS_FIXED_SIZE_1
   instdecoder.IS_SINGLE_MAX_OFFSET
   instdecoder.IS_DOUBLE_MAX_OFFSET
   instdecoder.IS_WORD_PRICING
   instdecoder.IS_BYTE_PRICING
   instdecoder.BILLING_PER_WORD
   instdecoder.BILLING_PER_BYTE
  )
  ;; source selector
  (mxp-into-instruction-decoder-selector)
  ;; source columns
  (
   1
   mxp.decoder/INST
   mxp.decoder/IS_MSIZE
   mxp.decoder/IS_RETURN
   mxp.decoder/IS_MCOPY
   mxp.decoder/IS_FIXED_SIZE_32
   mxp.decoder/IS_FIXED_SIZE_1
   mxp.decoder/IS_SINGLE_MAX_OFFSET
   mxp.decoder/IS_DOUBLE_MAX_OFFSET
   mxp.decoder/IS_WORD_PRICING
   mxp.decoder/IS_BYTE_PRICING
   mxp.decoder/G_WORD
   mxp.decoder/G_BYTE
  )
)
