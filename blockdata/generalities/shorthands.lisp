(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  2.X Shorthands          ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (flag-sum)    (force-bin   (+ IS_CB
					IS_TS
					IS_NB
					IS_PR
					IS_GL
					IS_ID
					IS_BF
					IS_BL)))

(defun    (wght-sum)    (+ ( * (^ 2 0) IS_CB)
			   ( * (^ 2 1) IS_TS)
			   ( * (^ 2 2) IS_NB)
			   ( * (^ 2 3) IS_PR)
			   ( * (^ 2 4) IS_GL)
			   ( * (^ 2 5) IS_ID)
			   ( * (^ 2 6) IS_BF)
			   ( * (^ 2 7) IS_BL))) ;; ""

(defun    (inst-sum)    (+ (* EVM_INST_COINBASE       IS_CB)
			   (* EVM_INST_TIMESTAMP      IS_TS)
			   (* EVM_INST_NUMBER         IS_NB)
			   (* EVM_INST_PREVRANDAO     IS_PR)
			   (* EVM_INST_GASLIMIT       IS_GL)
			   (* EVM_INST_CHAINID        IS_ID)
			   (* EVM_INST_BASEFEE        IS_BF)
			   (* EVM_INST_BLOBBASEFEE    IS_BL)))

(defun    (ct-max-sum)    (+ (* (- nROWS_CB 1) IS_CB)
			     (* (- nROWS_TS 1) IS_TS)
			     (* (- nROWS_NB 1) IS_NB)
			     (* (- nROWS_PV 1) IS_PR)
			     (* (- nROWS_GL 1) IS_GL)
			     (* (- nROWS_ID 1) IS_ID)
			     (* (- nROWS_BF 1) IS_BF)
			     (* (- nROWS_BL 1) IS_BL)))

(defconst
  nROWS_CB       1
  nROWS_TS       2
  nROWS_NB       2
  nROWS_PV       1
  nROWS_GL       5
  nROWS_ID       1
  nROWS_BF       1
  nROWS_BL       1
  nROWS_DEPTH    (+ nROWS_CB
		    nROWS_TS
		    nROWS_NB
		    nROWS_PV
		    nROWS_GL
		    nROWS_ID
		    nROWS_BF
		    nROWS_BL)
  )

(defun    (upcoming-phase-is-different)    (+ (* (- 1 IS_CB) (next IS_CB))
					      (* (- 1 IS_TS) (next IS_TS))
					      (* (- 1 IS_NB) (next IS_NB))
					      (* (- 1 IS_PR) (next IS_PR))
					      (* (- 1 IS_GL) (next IS_GL))
					      (* (- 1 IS_ID) (next IS_ID))
					      (* (- 1 IS_BF) (next IS_BF))
					      (* (- 1 IS_BL) (next IS_BL))))

(defun    (upcoming-phase-is-the-same)     (+ (* IS_CB (next IS_CB))
					      (* IS_TS (next IS_TS))
					      (* IS_NB (next IS_NB))
					      (* IS_PR (next IS_PR))
					      (* IS_GL (next IS_GL))
					      (* IS_ID (next IS_ID))
					      (* IS_BF (next IS_BF))
					      (* IS_BL (next IS_BL))))

(defun    (upcoming-legal-phase-transition)     (+ (* IS_CB (next IS_TS))
						   (* IS_TS (next IS_NB))
						   (* IS_NB (next IS_PR))
						   (* IS_PR (next IS_GL))
						   (* IS_GL (next IS_ID))
						   (* IS_ID (next IS_BF))
						   (* IS_BF (next IS_BL))
						   (* IS_BL (next IS_CB))))

(defun  (isnt-first-block-in-conflation)  (shift  IOMF  (-  0   nROWS_DEPTH)))
(defun  (is-first-block-in-conflation)    (force-bin    (-  1  (isnt-first-block-in-conflation))))
(defun  (curr-data-hi)                            DATA_HI                     )
(defun  (curr-data-lo)                            DATA_LO                     )
(defun  (prev-data-hi)                    (shift  DATA_HI  (- 0 nROWS_DEPTH)))
(defun  (prev-data-lo)                    (shift  DATA_LO  (- 0 nROWS_DEPTH)))

