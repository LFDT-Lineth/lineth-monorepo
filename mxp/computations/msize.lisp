(module mxp)


(defconstraint  computation---MSIZE-scenario---setting-macro-RES
		(:guard (mxp-guard---msize))
		(begin
		  (eq!        (shift  macro/RES   NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO)  scenario/WORDS)
		  (vanishes!  (shift  macro/MXPX  NEGATIVE_ROW_OFFSET___SCNRI_TO_MACRO))
		  ))

(defconstraint  computation---MSIZE-scenario---both-module-lookup-flags-are-off
		(:guard (mxp-guard---msize))
		(debug
		  (vanishes!   (shift  (+   computation/WCP_FLAG   computation/EUC_FLAG)   1))))
