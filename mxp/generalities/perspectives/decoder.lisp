(module mxp)

(defconstraint  generalities---perspectives---decoder---MSIZE-vanishings
		(:guard DECODER)
		(if-not-zero   decoder/IS_MSIZE
			       (begin
				 (vanishes!   (shift   macro/OFFSET_1_HI       ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/OFFSET_1_LO       ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/SIZE_1_HI         ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/SIZE_1_LO         ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/S1NZNOMXPX        ROW_OFFSET___DECDR_TO_MACRO)) ;; end of 1st parameter set
				 (vanishes!   (shift   macro/OFFSET_2_HI       ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/OFFSET_2_LO       ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/SIZE_2_HI         ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/SIZE_2_LO         ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/S2NZNOMXPX        ROW_OFFSET___DECDR_TO_MACRO)) ;; end of 2nd parameter set
				 (vanishes!   (shift   macro/MXPX              ROW_OFFSET___DECDR_TO_MACRO))
				 (vanishes!   (shift   macro/GAS_MXP           ROW_OFFSET___DECDR_TO_MACRO))
				 )))

(defconstraint  generalities---perspectives---decoder---RES-is-zero-if-not-MSIZE
		(:guard DECODER)
		(if-zero   decoder/IS_MSIZE
			   (vanishes!   (shift   macro/RES   ROW_OFFSET___DECDR_TO_MACRO))))

(defconstraint  generalities---perspectives---decoder---imposing-SIZE_1-for-is-fixed-size-instructions
		(:guard DECODER)
		(begin
		  (if-not-zero   decoder/IS_FIXED_SIZE_32
				 (begin
				   (eq!   (shift   macro/SIZE_1_HI   ROW_OFFSET___DECDR_TO_MACRO)    0)
				   (eq!   (shift   macro/SIZE_1_LO   ROW_OFFSET___DECDR_TO_MACRO)   32)))
		  (if-not-zero   decoder/IS_FIXED_SIZE_1
				 (begin
				   (eq!   (shift   macro/SIZE_1_HI   ROW_OFFSET___DECDR_TO_MACRO)    0)
				   (eq!   (shift   macro/SIZE_1_LO   ROW_OFFSET___DECDR_TO_MACRO)    1)))
		  ))


(defconstraint  generalities---perspectives---decoder---2nd-parameters-vanish-for-single-parameter-set-instructions
		(:guard DECODER)
		(if-zero   decoder/IS_DOUBLE_MAX_OFFSET
			   (begin
			     (vanishes!   (shift   macro/OFFSET_2_HI       ROW_OFFSET___DECDR_TO_MACRO))
			     (vanishes!   (shift   macro/OFFSET_2_LO       ROW_OFFSET___DECDR_TO_MACRO))
			     (vanishes!   (shift   macro/SIZE_2_HI         ROW_OFFSET___DECDR_TO_MACRO))
			     (vanishes!   (shift   macro/SIZE_2_LO         ROW_OFFSET___DECDR_TO_MACRO))
			     (vanishes!   (shift   macro/S2NZNOMXPX        ROW_OFFSET___DECDR_TO_MACRO)) ;; end of 2nd parameter set
			     )))

(defconstraint  generalities---perspectives---decoder---MCOPY-expects-equal-first-and-second-size-parameters
		(:guard DECODER)
		(if-not-zero   decoder/IS_MCOPY
			       (begin
				 (eq!  (shift   macro/SIZE_1_HI   ROW_OFFSET___DECDR_TO_MACRO)
				       (shift   macro/SIZE_2_HI   ROW_OFFSET___DECDR_TO_MACRO))
				 (eq!  (shift   macro/SIZE_1_LO   ROW_OFFSET___DECDR_TO_MACRO)
				       (shift   macro/SIZE_2_LO   ROW_OFFSET___DECDR_TO_MACRO))
				 )))
