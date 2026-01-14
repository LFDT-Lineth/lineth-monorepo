(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   X.Y The XXX_TXN_NUMBER columns         ;;
;;   X.Y.Z Shorthands for transaction end   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defcomputedcolumn  (TOTL_TXN_NUMBER :i24)    (+    SYSI_TXN_NUMBER
						    USER_TXN_NUMBER
						    SYSF_TXN_NUMBER))

(defconstraint    system-txn-numbers---initialization (:domain {0}) ;; ""
		  (begin
		    (vanishes!    SYSI_TXN_NUMBER)
		    (vanishes!    USER_TXN_NUMBER)
		    (vanishes!    SYSF_TXN_NUMBER)
		    ))

;; TODO: this should be a (defproperty ...) declaration, but the domain seems to be the issue
(defconstraint    system-txn-numbers---initialization---TOTL (:domain {0}) ;; ""
		  (vanishes!    TOTL_TXN_NUMBER))


(defconstraint    system-txn-numbers---increments ()
		  (begin
		    (did-inc!    SYSI_TXN_NUMBER    (system-txn-numbers---sysi-txn-start))
		    (did-inc!    USER_TXN_NUMBER    (system-txn-numbers---user-txn-start))
		    (did-inc!    SYSF_TXN_NUMBER    (system-txn-numbers---sysf-txn-start))
		    ))

(defproperty      system-txn-numbers---increments---TOTL
		  (did-inc!    TOTL_TXN_NUMBER    (system-txn-numbers---txn-start)))

(defproperty      system-txn-numbers---0-1-increments
		  (begin
		    (has-0-1-increments    SYSI_TXN_NUMBER)
		    (has-0-1-increments    USER_TXN_NUMBER)
		    (has-0-1-increments    SYSF_TXN_NUMBER)
		    (has-0-1-increments    TOTL_TXN_NUMBER)
		    ))
