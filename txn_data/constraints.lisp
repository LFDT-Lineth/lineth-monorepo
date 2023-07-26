(module txn_data)

(defconst
	nROWS_TYPE_0 6
	nROWS_TYPE_1 7
	nROWS_TYPE_2 7)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 Heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint first_row (:domain {0})
	       (vanishes! ABS))

;; (defconstraint heartbeat ()
;; 	       (begin
;; 		 (* (will-rmain-constant! STAMP) (will-inc! STAMP 1))
;; 		 (if-not-zero (will-remain-constant! STAMP) (vanishes! (next CT)))
;; 		 (if-not-zero STAMP
;; 			      (if-eq-else CT LLARGEMO
;; 					  (will-inc! STAMP 1)
;; 					  (will-inc! CT 1)))))

(defconstraint final_row (:domain {-1})
	       (vanishes! ABS))
