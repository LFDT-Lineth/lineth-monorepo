(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   10.2 SCEN/RETURN instruction shorthands   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; ;;  RETURN/message_call
(defun (scenario-shorthand---RETURN---message-call)
  (+ ;; scenario/RETURN_EXCEPTION
     scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
     scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM
     ;; scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
     ))

;; ;;  RETURN/empty_deployment
(defun (scenario-shorthand---RETURN---empty-deployment)
  (+ ;; scenario/RETURN_EXCEPTION
     ;; scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
     ;; scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM
     scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
     scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
     ))

;; ;;  RETURN/nonempty_deployment
(defun (scenario-shorthand---RETURN---nonempty-deployment)
  (+ ;; scenario/RETURN_EXCEPTION
     ;; scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
     ;; scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM
     ;; scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
     scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
     scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
     ))

;; ;;  RETURN/deployment
(defun (scenario-shorthand---RETURN---deployment)
  (+ (scenario-shorthand---RETURN---empty-deployment)
     (scenario-shorthand---RETURN---nonempty-deployment)))

;; ;;  RETURN/deployment_will_revert
(defun (scenario-shorthand---RETURN---deployment-will-revert)
  (+ ;; scenario/RETURN_EXCEPTION
     ;; scenario/RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
     ;; scenario/RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM
     scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT
     scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
     ;; scenario/RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
     ))

;; ;; NOT A CONSTRAINT
;; (defconstraint  BULLSHIT-making-sure-everything-compiles-scenario-shorthand-RETURNs ()
;;                (begin  (vanishes! (scenario-shorthand---RETURN---message-call)           )
;;                        (vanishes! (scenario-shorthand---RETURN---empty-deployment)       )
;;                        (vanishes! (scenario-shorthand---RETURN---nonempty-deployment)    )
;;                        (vanishes! (scenario-shorthand---RETURN---deployment)             )
;;                        (vanishes! (scenario-shorthand---RETURN---deployment-will-revert) )
;;                        (vanishes! (scenario-shorthand---RETURN---unexceptional)          )
;;                        (vanishes! (scenario-shorthand---RETURN---sum)                    )))

;; ;;  RETURN/unexceptional
(defun (scenario-shorthand---RETURN---unexceptional)
  (+ (scenario-shorthand---RETURN---message-call)
     (scenario-shorthand---RETURN---deployment)))

;; ;;  RETURN/sum
(defun (scenario-shorthand---RETURN---sum)
  (+ scenario/RETURN_EXCEPTION
     (scenario-shorthand---RETURN---unexceptional)))
