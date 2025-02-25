(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    X.Y.7 Triggers   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (create-instruction---trigger_MXP)        (+    (create-instruction---STACK-mxpx)
                                                          (create-instruction---STACK-oogx)
                                                          (scenario-shorthand---CREATE---unexceptional)))

(defun    (create-instruction---trigger_STP)        (+    (create-instruction---STACK-oogx)
                                                          (scenario-shorthand---CREATE---unexceptional)))

(defun    (create-instruction---trigger_OOB)        (+    (scenario-shorthand---CREATE---unexceptional)))

(defun    (create-instruction---trigger_MMU)        (+    (create-instruction---hash-init-code)
                                                          (create-instruction---hash-init-code-and-send-to-ROM)
                                                          (create-instruction---send-init-code-to-ROM)))

(defun    (create-instruction---trigger_HASHINFO)   (*    (create-instruction---is-CREATE2)
                                                          (+    (*    (scenario-shorthand---CREATE---failure-condition)
                                                                      (create-instruction---MXP-mtntop))
                                                                (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code))))

(defun    (create-instruction---trigger_RLPADDR)    (scenario-shorthand---CREATE---compute-deployment-address))
(defun    (create-instruction---trigger_ROMLEX)     (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code))


;; auxiliary shorthands required for (create-instruction---trigger_MMU)
(defun    (create-instruction---hash-init-code)                   (*    (scenario-shorthand---CREATE---failure-condition)
                                                                        (create-instruction---MXP-mtntop)
                                                                        (create-instruction---is-CREATE2)))

(defun    (create-instruction---hash-init-code-and-send-to-ROM)   (*    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
                                                                        (create-instruction---is-CREATE2)))

(defun    (create-instruction---send-init-code-to-ROM)            (*    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
                                                                        (create-instruction---is-CREATE)))
