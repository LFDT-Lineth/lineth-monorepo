(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                   ;;
;;   X.Y The HUB_STAMP column                        ;;
;;   X.Y.Z The TLI, CT_TLI and NSR, CT_NSR columns   ;;
;;                                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    system-counters---hub-stamp-constancy-of-TLI-and-NSR ()
		  (begin
		    (hub-stamp-constancy    TLI)
		    (hub-stamp-constancy    NSR)))

(defconstraint    system-counters---automatic-vanishing-constraints-for-max-counters ()
		  (if-zero    TX_EXEC
			      (begin
				(vanishes!    TLI)
				(vanishes!    NSR))))

(defconstraint    system-counters---automatic-vanishing-constraints-for-counters---at-HUB_STAMP-transitions ()
		  (if-not    (remained-constant!    HUB_STAMP)
			     (begin
			       (vanishes!    COUNTER_TLI)
			       (vanishes!    COUNTER_NSR))))

(defproperty      system-counters---automatic-vanishing-constraints-for-counters---outside-of-execution-rows
		  (if-zero    TX_EXEC
			      (begin
				(vanishes!    COUNTER_TLI)
				(vanishes!    COUNTER_NSR))))

(defconstraint    system-counters---progression-constraints ()
		  (if-not-zero    TX_EXEC
				  (if-not-zero    (- COUNTER_TLI TLI)
						  ;; COUNTER_TLI ≠ TLI
						  (begin
						    (will-inc!               COUNTER_TLI    1)
						    (will-remain-constant!   COUNTER_NSR)
						    (vanishes!               COUNTER_NSR))
						  ;; COUNTER_TLI = TLI
						  (if-not-zero    (-    COUNTER_NSR    NSR)
								  ;; COUNTER_NSR ≠ NSR
								  (begin
								    (will-remain-constant!   COUNTER_TLI)
								    (will-inc!               COUNTER_NSR    1)
								    )
								  ;; COUNTER_NSR = NSR
								  (will-inc!    HUB_STAMP    1)
								  ))))

(defconstraint    system-counters---pegging-CT_NSR-to-PEEK_AT_STACK ()
		  (if-not-zero    TX_EXEC
				  (if-zero    COUNTER_NSR
					      ;; COUNTER_NSR = 0
					      (eq!    PEEK_AT_STACK    1)
					      ;; COUNTER_NSR ≠ 0
					      (eq!    PEEK_AT_STACK    0)
					      )))
