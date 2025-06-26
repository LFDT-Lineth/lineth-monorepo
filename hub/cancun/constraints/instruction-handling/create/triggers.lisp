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

(defun    (create-instruction---trigger_OOB_X)      (+    (create-instruction---STACK-maxcsx)           ))
(defun    (create-instruction---trigger_OOB_U)      (+    (scenario-shorthand---CREATE---unexceptional) ))
(defun    (create-instruction---trigger_OOB)        (+    (create-instruction---trigger_OOB_X)
                                                          (create-instruction---trigger_OOB_U)          ))

(defun    (create-instruction---trigger_MMU)        (+    (create-instruction---hash-init-code)
                                                          (create-instruction---hash-init-code-and-send-to-ROM)
                                                          (create-instruction---send-init-code-to-ROM)))

(defun    (create-instruction---trigger_HASHINFO)   (*    (create-instruction---is-CREATE2)
                                                          (+    (*    (scenario-shorthand---CREATE---failure-condition)
                                                                      (create-instruction---MXP-s1nznomxpx))
                                                                (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code))))

(defun    (create-instruction---trigger_RLPADDR)    (scenario-shorthand---CREATE---compute-deployment-address))
(defun    (create-instruction---trigger_ROMLEX)     (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code))


;; auxiliary shorthands required for (create-instruction---trigger_MMU)
(defun    (create-instruction---hash-init-code)                   (*    (scenario-shorthand---CREATE---failure-condition)
                                                                        (create-instruction---MXP-s1nznomxpx)
                                                                        (create-instruction---is-CREATE2)))

(defun    (create-instruction---hash-init-code-and-send-to-ROM)   (*    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
                                                                        (create-instruction---is-CREATE2)))

(defun    (create-instruction---send-init-code-to-ROM)            (*    (scenario-shorthand---CREATE---not-rebuffed-nonempty-init-code)
                                                                        (create-instruction---is-CREATE)))
