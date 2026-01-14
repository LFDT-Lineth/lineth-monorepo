(module mxp)

(defun  (euc-call  relof
		   a
		   b)
  (begin
    (eq!  (shift  computation/EUC_FLAG  relof)  1)
    (eq!  (shift  computation/ARG_1_LO  relof)  a)
    (eq!  (shift  computation/ARG_2_LO  relof)  b)))


(defun  (wcp-call-to-LT  relof
			 arg_1_hi
			 arg_1_lo
			 arg_2_hi
			 arg_2_lo)
  (begin
    (eq!  (shift  computation/WCP_FLAG  relof)  1           )
    (eq!  (shift  computation/ARG_1_HI  relof)  arg_1_hi    )
    (eq!  (shift  computation/ARG_1_LO  relof)  arg_1_lo    )
    (eq!  (shift  computation/ARG_2_HI  relof)  arg_2_hi    )
    (eq!  (shift  computation/ARG_2_LO  relof)  arg_2_lo    )
    (eq!  (shift  computation/EXO_INST  relof)  EVM_INST_LT )))


(defun  (wcp-call-to-LEQ  relof
			  arg_1_hi
			  arg_1_lo
			  arg_2_hi
			  arg_2_lo)
  (begin
    (eq!  (shift  computation/WCP_FLAG  relof)  1            )
    (eq!  (shift  computation/ARG_1_HI  relof)  arg_1_hi     )
    (eq!  (shift  computation/ARG_1_LO  relof)  arg_1_lo     )
    (eq!  (shift  computation/ARG_2_HI  relof)  arg_2_hi     )
    (eq!  (shift  computation/ARG_2_LO  relof)  arg_2_lo     )
    (eq!  (shift  computation/EXO_INST  relof)  WCP_INST_LEQ )))


(defun  (wcp-call-to-ISZERO  relof
			     arg_1_hi
			     arg_1_lo)
  (begin
    (eq!  (shift  computation/WCP_FLAG  relof)  1               )
    (eq!  (shift  computation/ARG_1_HI  relof)  arg_1_hi        )
    (eq!  (shift  computation/ARG_1_LO  relof)  arg_1_lo        )
    (eq!  (shift  computation/ARG_2_HI  relof)  0               )
    (eq!  (shift  computation/ARG_2_LO  relof)  0               )
    (eq!  (shift  computation/EXO_INST  relof)  EVM_INST_ISZERO )))
