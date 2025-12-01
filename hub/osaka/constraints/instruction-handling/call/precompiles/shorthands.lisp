(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;    X.Y.Z.1 Introduction                         ;;
;;    X.Y.Z.2 Precompile failure vs. success       ;;
;;    X.Y.Z.3 Precompile failure classification    ;;
;;    X.Y.Z.4 Global shorthands                    ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---dup-caller-gas)    scenario/PRC_CALLER_GAS)
(defun    (precompile-processing---dup-callee-gas)    scenario/PRC_CALLEE_GAS)
(defun    (precompile-processing---prd-return-gas)    scenario/PRC_RETURN_GAS)
(defun    (precompile-processing---dup-cdo)           scenario/PRC_CDO)
(defun    (precompile-processing---dup-cds)           scenario/PRC_CDS)
(defun    (precompile-processing---dup-r@o)           scenario/PRC_RAO)
(defun    (precompile-processing---dup-r@c)           scenario/PRC_RAC)
