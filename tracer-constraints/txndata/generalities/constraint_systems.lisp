(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                    ;;
;;    X.Y.Z WCP_FLAG, EUC_FLAG and parametrized constraint systems    ;;
;;                                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; binary constraints for WCP and EUC taken care of via :binary@prove

(defconstraint   EUC-and-WCP-are-binary-exclusive (:perspective computation)
                 (vanishes!   (*   EUC_FLAG   WCP_FLAG)))

(defun (small-call-to-LT   relOffset
                           arg1
                           arg2)
  (begin
    (eq!  (shift  computation/WCP_FLAG  relOffset)  1)
    (eq!  (shift  computation/ARG_1_LO  relOffset)  arg1)
    (eq!  (shift  computation/ARG_2_LO  relOffset)  arg2)
    (eq!  (shift  computation/INST      relOffset)  EVM_INST_LT)))

(defun (small-call-to-LEQ   relOffset
                            arg1
                            arg2)
  (begin
    (eq!   (shift   computation/WCP_FLAG   relOffset)   1)
    (eq!   (shift   computation/ARG_1_LO   relOffset)   arg1)
    (eq!   (shift   computation/ARG_2_LO   relOffset)   arg2)
    (eq!   (shift   computation/INST       relOffset)   WCP_INST_LEQ)))

(defun (small-call-to-ISZERO   relOffset
                               arg1)
  (begin
    (eq!  (shift  computation/WCP_FLAG  relOffset)  1)
    (eq!  (shift  computation/ARG_1_LO  relOffset)  arg1)
    (eq!  (shift  computation/INST      relOffset)  EVM_INST_ISZERO)))

(defun (call-to-EUC   relOffset
                      arg1
                      arg2)
  (begin
    (eq!  (shift  computation/EUC_FLAG  relOffset)  1)
    (eq!  (shift  computation/ARG_1_LO  relOffset)  arg1)
    (eq!  (shift  computation/ARG_2_LO  relOffset)  arg2)))

(defun   (result-must-be-true    relOffset)   (eq!   (shift   computation/WCP_RES   relOffset)   1))
(defun   (result-must-be-false   relOffset)   (eq!   (shift   computation/WCP_RES   relOffset)   0))
