(module mxp)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                               ;;
;;   SIZE_1/2 smallness checks   ;;
;;                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint  computations---not-msize-nor-trivial---smallness-test-for-SIZE_1
		(:guard   (mxp-guard---not-msize-not-trivial))
		(wcp-call-to-LEQ   ROW_OFFSET___1ST_SIZE___SMALLNESS_TEST
				   (mxp-shorthand---size-1-hi)
				   (mxp-shorthand---size-1-lo)
				   0
				   CANCUN_MXPX_THRESHOLD
				   ))

(defconstraint  computations---not-msize-nor-trivial---smallness-test-for-SIZE_2
		(:guard   (mxp-guard---not-msize-not-trivial))
		(wcp-call-to-LEQ   ROW_OFFSET___2ND_SIZE___SMALLNESS_TEST
				   (mxp-shorthand---size-2-hi)
				   (mxp-shorthand---size-2-lo)
				   0
				   CANCUN_MXPX_THRESHOLD
				   ))


(defun   (mxp-shorthand---size-1-is-small)   (shift computation/RES_A ROW_OFFSET___1ST_SIZE___SMALLNESS_TEST))
(defun   (mxp-shorthand---size-2-is-small)   (shift computation/RES_A ROW_OFFSET___2ND_SIZE___SMALLNESS_TEST))
(defun   (mxp-shorthand---size-1-is-large)   (-  1  (mxp-shorthand---size-1-is-small)))
(defun   (mxp-shorthand---size-2-is-large)   (-  1  (mxp-shorthand---size-2-is-small)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;   OFFSET_1/2 smallness checks   ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint  computations---not-msize-nor-trivial---smallness-test-for-OFFSET_1
		(:guard   (mxp-guard---not-msize-not-trivial))
		(wcp-call-to-LEQ   ROW_OFFSET___1ST_OFFSET___SMALLNESS_TEST
				   (mxp-shorthand---offset-1-hi)
				   (mxp-shorthand---offset-1-lo)
				   0
				   CANCUN_MXPX_THRESHOLD
				   ))

(defconstraint  computations---not-msize-nor-trivial---smallness-test-for-OFFSET_2
		(:guard   (mxp-guard---not-msize-not-trivial))
		(wcp-call-to-LEQ   ROW_OFFSET___2ND_OFFSET___SMALLNESS_TEST
				   (mxp-shorthand---offset-2-hi)
				   (mxp-shorthand---offset-2-lo)
				   0
				   CANCUN_MXPX_THRESHOLD
				   ))


(defun   (mxp-shorthand---offset-1-is-small)   (shift computation/RES_A ROW_OFFSET___1ST_OFFSET___SMALLNESS_TEST))
(defun   (mxp-shorthand---offset-2-is-small)   (shift computation/RES_A ROW_OFFSET___2ND_OFFSET___SMALLNESS_TEST))
(defun   (mxp-shorthand---offset-1-is-large)   (-  1  (mxp-shorthand---offset-1-is-small)))
(defun   (mxp-shorthand---offset-2-is-large)   (-  1  (mxp-shorthand---offset-2-is-small)))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;   Scenario guards   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (mxp-shorthand---mxpx-expression-1) (+  (mxp-shorthand---size-1-is-large)
						 (*   (mxp-shorthand---size-1-is-nonzero)
						      (mxp-shorthand---offset-1-is-large))))

(defun   (mxp-shorthand---mxpx-expression-2) (+  (mxp-shorthand---size-2-is-large)
						 (*   (mxp-shorthand---size-2-is-nonzero)
						      (mxp-shorthand---offset-2-is-large))))

(defun   (mxp-shorthand---mxpx-expression)   (+  (mxp-shorthand---mxpx-expression-1)
						 (mxp-shorthand---mxpx-expression-2)))

(defconstraint  computations---not-msize-nor-trivial---justifying-the-MXPX-scenario-flag
		(:guard    (mxp-guard---not-msize-not-trivial))
		(if-zero   (mxp-shorthand---mxpx-expression)
			   ;; zero case
			   (eq!   scenario/MXPX   0)
			   ;; nonzero case
			   (eq!   scenario/MXPX   1)
			   ))
