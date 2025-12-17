(module txndata)



(defun
  (block-constancy   COL)
  (if-not-zero   (-   (next BLK_NUMBER)   (+  1  BLK_NUMBER))
		 (eq! (next  COL)  COL)))
(defun
  (conflation-constancy   COL)
  (if-not-zero   (perspective-sum)
		 (eq!  (next  COL)  COL)))
(defun
  (transaction-constancy   COL)
  (if-not-zero   (-    (next  TOTL_TXN_NUMBER)   (+  1  TOTL_TXN_NUMBER))
		 (eq!  (next  COL)  COL)))
(defun
  (user-transaction-constancy   COL)
  (if-not-zero   (*    (next  USER )  USER )
		 (eq!  (next  COL  )  COL  )))




(defcomputedcolumn      ( prover___RELATIVE_USER_TXN_NUMBER :i16 :fwd )
			(if-eq-else     BLK_NUMBER   (prev   BLK_NUMBER)
					;; BLK# equality case
					(*   USER   (+   (prev   prover___RELATIVE_USER_TXN_NUMBER) (-   USER_TXN_NUMBER (prev   USER_TXN_NUMBER))))
					;; BLK# change case
					0
					))

(defcomputedcolumn      ( prover___IS_LAST_USER_TXN_OF_BLOCK :binary@prove :bwd )
			(if-eq-else     (next   TOTL_TXN_NUMBER)   TOTL_TXN_NUMBER
					;; TOTL transaction number equality case
					(next   prover___IS_LAST_USER_TXN_OF_BLOCK)
					;; BLK# change case
					(*   (next   SYSF)   USER)
					))

(defalias
  RELUSR   prover___RELATIVE_USER_TXN_NUMBER
  LSTUSR   prover___IS_LAST_USER_TXN_OF_BLOCK
  )



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                           ;;
;;   Property verification   ;;
;;                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defproperty    prover-column-constraints---RELATIVE_USER_TXN_NUMBER---const         (transaction-constancy   RELUSR))
(defproperty    prover-column-constraints---RELATIVE_USER_TXN_NUMBER---vanish        (if-zero  (perspective-sum) (vanishes!   RELUSR)))
(defproperty    prover-column-constraints---RELATIVE_USER_TXN_NUMBER---transition    (if-not-zero   (-  (next  BLK_NUMBER)  BLK_NUMBER) (vanishes!   RELUSR)))
(defproperty    prover-column-constraints---RELATIVE_USER_TXN_NUMBER---increment     (if-not-zero   (-  (next  BLK_NUMBER)  (+  BLK_NUMBER  1))
												    (eq!   (next  RELUSR)
													   (*   (next   USER)   (+   RELUSR (-   (next   USER_TXN_NUMBER) USER_TXN_NUMBER))))))

(defproperty    prover-column-constraints---IS_LAST_USER_TXN_OF_BLOCK---const    (transaction-constancy   LSTUSR) )
(defproperty    prover-column-constraints---IS_LAST_USER_TXN_OF_BLOCK---vanish   (if-zero   USER (vanishes!   LSTUSR)) )
(defproperty    prover-column-constraints---IS_LAST_USER_TXN_OF_BLOCK---last     (if-not-zero   (-  TOTL_TXN_NUMBER   (prev  TOTL_TXN_NUMBER))
												(eq!   (prev   LSTUSR)
												       (*   (prev   USER)   SYSF)))
		)



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   Constraints verification   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defcolumns
  ( prover___RELATIVE_USER_TXN_NUMBER_MAX :i16 )
  ( prover___USER_TXN_NUMBER_MAX          :i16 )
  )

(defalias
  RELMAX   prover___RELATIVE_USER_TXN_NUMBER_MAX
  USRMAX   prover___USER_TXN_NUMBER_MAX
  )



(defconstraint  prover-column-constraints---RELATIVE_USER_TXN_NUMBER_MAX---const   () (block-constancy   RELMAX))
(defconstraint  prover-column-constraints---RELATIVE_USER_TXN_NUMBER_MAX---zero    () (if-zero           (perspective-sum)
													 (vanishes!   RELMAX)))
(defconstraint  prover-column-constraints---RELATIVE_USER_TXN_NUMBER_MAX---setting () (if-not-zero       (*  (- 1 (prev SYSF))  SYSF)
													 (eq!   (prev RELMAX)   (prev  RELUSR))))

(defconstraint  prover-column-constraints---USER_TXN_NUMBER_MAX ()
		(begin
		  (conflation-constancy   USRMAX)
		  (if-zero   (perspective-sum)
			     (vanishes!   USRMAX))
		  ))

(defconstraint  prover-column-constraints---USER_TXN_NUMBER_MAX---finalization
		(:domain {-1}) ;; ""
		(eq!   USRMAX   USER_TXN_NUMBER))



;; TOTL|USER|SYSF|SYSI|prover
