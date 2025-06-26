(defun (mxp-into-instruction-decoder-selector) mxp.DECODER) ;; ""

(deflookup hub-into-instdecoder
           ;;
           ;; target columns
           ;;
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
           ;;
           ;; source columns
           ;;
	   (
	     (* 1                                   (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/INST                 (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_MSIZE             (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_RETURN            (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_MCOPY             (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_FIXED_SIZE_32     (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_FIXED_SIZE_1      (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_SINGLE_MAX_OFFSET (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_DOUBLE_MAX_OFFSET (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_WORD_PRICING      (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/IS_BYTE_PRICING      (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/G_WORD               (mxp-into-instruction-decoder-selector))
	     (* mxp.decoder/G_BYTE               (mxp-into-instruction-decoder-selector))
	     ;;
           )
)
