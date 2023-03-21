(module mmio)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;              4.2 Single byte swap               ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;ByteSwap
(defun (byte-swap
		S T T_NEW
		SB TB
		ACC P
		TM BIT_1 BIT_2 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 TM CT)
					(plateau BIT_2 (+ TM 1) CT)
					(isolate-chunk ACC TB BIT_1 BIT_2 CT)
					(power P BIT_2 CT)
    				(if-eq CT LLARGEMO
						(eq T_NEW (+ (* (- SB ACC) P) T)))))
;======================================================================




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;;              4.3 Excision               ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;Excision
(defun (excision
		T T_NEW
		TB
		ACC
		P
		TM SIZE
		BIT_1 BIT_2 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 TM CT)
					(plateau BIT_2 (+ TM SIZE) CT)
					(isolate-chunk ACC TB BIT_1 BIT_2 CT)
					(power P BIT_2 CT)
					(if-eq CT LLARGEMO
						(= T_NEW (- T (* ACC P))))))
;======================================================================




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;              4.4 [1 => 1 Padded]               ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[1 => 1 Padded]
(defun (one-to-one-padded
		S T
		SB
		ACC P
		SM SIZE
		BIT_1 BIT_2 BIT_3 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 SM CT)
					(plateau BIT_2 (+ SM SIZE) CT)
					(plateau BIT_3 SIZE CT)
					(isolate-chunk ACC SB BIT_1 BIT_2 CT)
					(power P BIT_3 CT)
					(if-eq CT LLARGEMO
						(= T (* ACC P)))))
;======================================================================




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                ;;
;;              4.5 [2 => 1 Padded]               ;;
;;                                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[2 => 1 Padded]
(defun (two-to-one-padded
		S1 S2 T
		S1B S2B
		ACC_1 ACC_2 P_1 P_2
		S1M SIZE
		BIT_1 BIT_2 BIT_3 BIT_4 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 S1M CT)
					(plateau BIT_2 (+ S1M (- SIZE LLARGE)) CT)
					(plateau BIT_3 (- LLARGE S1M) CT)
					(plateau BIT_4 SIZE CT)
					(isolate-suffix ACC_1 S1B BIT_1 CT)
					(isolate-prefix ACC_2 S2B BIT_2 CT)
					(power P_1 BIT_3 CT)
					(power P_2 BIT_4 CT)
					(if-eq CT LLARGEMO
						(= T 
							(+ 
								(* ACC_1 P_1)
								(* ACC_2 P_2))))))
;======================================================================



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                              ;;
;;              4.6 [1 Full => 2]               ;;
;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[1 Full => 2]
(defun (one-full-to-two
		S T1 T2 T1_NEW T2_NEW
		SB T1B T2B
		ACC1 ACC2 ACC3 ACC4 P
		T1M BIT1 BIT2 CT)
;______________________________________________________________________

				(begin
					(plateau BIT1 T1M CT)
					(plateau BIT2 (- LLARGE T1M) CT)
					(isolate-prefix ACC2 T2B BIT1 CT)
					(isolate-prefix ACC3  SB BIT2 CT)
					(isolate-suffix ACC4  SB BIT2 CT)
					(power P BIT1 CT)
					(if-eq CT LLARGEMO
						(= T1_NEW (- ACC3 ACC1))
						(= T2_NEW (* (- ACC4 ACC2) P)))))
;======================================================================




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                              ;;
;;              4.7 [2 => 1 Full]               ;;
;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[2 => 1 Full]
(defun (two-to-one-full
		S1 S2 T
		S1B S2B
		ACC_1 ACC_2 P
		SM BIT_1 BIT_2 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 SM CT)
					(plateau BIT_2 (- LLARGE SM) CT)
					(isolate-suffix ACC_1 S1B BIT_1 CT)
					(isolate-prefix ACC_2 S2B BIT_1 CT)
					(power P BIT_2 CT)
					(if-eq CT LLARGEMO
						(begin
							(eq T (+ (* P ACC_1) ACC_2))))))
;======================================================================




;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;              4.8 [1 Partial => 1]               ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[1 Partial => 1]
(defun (one-partial-to-one
		S T T_NEW
		SB TB
		ACC_1 ACC_2 P
		SM TM SIZE
		BIT_1 BIT_2 BIT_3 BIT_4 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 TM CT)
					(plateau BIT_2 (+ TM SIZE) CT)
					(plateau BIT_3 SM CT)
					(plateau BIT_4 (+ SM SIZE) CT)
					(isolate-chunk ACC_1 TB BIT_1 BIT_2 CT)
					(isolate-chunk ACC_2 SB BIT_3 BIT_4 CT)
					(power P BIT_2 CT)
					(if-eq CT LLARGEMO
						(= T_NEW (+ T (* (- ACC_2 ACC_1) P))))))
;======================================================================



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;              4.9 [1 Partial => 2]               ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[1 Partial => 2]
(defun (one-partial-to-two 
		S T1 T2 T1_NEW T2_NEW 
		SB T1B T2B
		ACC_1 ACC_2 ACC_3 ACC_4 P
		SM T1M SIZE
		BIT_1 BIT_2 BIT_3 BIT_4 BIT_5 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 T1M CT)
					(plateau BIT_2 (- (+ T1M SIZE) LLARGE) CT)
					(plateau BIT_3 SM CT)
					(plateau BIT_4 (- (+ SM LLARGE) T1M) CT)
					(plateau BIT_5 (+ SM SIZE) CT)
					(isolate-suffix ACC_1 T1B BIT_1 CT)
					(isolate-prefix ACC_2 T2B BIT_2 CT)
					(isolate-chunk ACC_3 SB BIT_3 BIT_4 CT)
					(isolate-chunk ACC_4 SB BIT_4 BIT_5 CT)
					(power P BIT_2 CT)
					(if-eq CT LLARGEMO
      					(begin
							(eq T1_NEW (+ T1 (- ACC_3 ACC_1)))
							(eq T2_NEW (+ T2 (* (- ACC_4 ACC_2) P)))))))
;======================================================================



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                              ;;
;;              4.9 [2 Full => 3]               ;;
;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[2 Full => 3]
(defun (two-full-to-three
		T1 T3
		S1 S2
		T1_NEW T2_NEW T3_NEW
		T1B T3B
		S1B S2B
		ACC_1 ACC_2 ACC_3 ACC_4 ACC_5 ACC_6
		P TM  BIT_1 BIT_2 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 TM CT)
					(plateau BIT_2 (- LLARGE TM) CT)
					(isolate-suffix ACC_1 T1B BIT_1 CT)
					(isolate-prefix ACC_2 T3B BIT_1 CT)
					(isolate-prefix ACC_3 S1B BIT_2 CT)
					(isolate-suffix ACC_4 S1B BIT_2 CT)
					(isolate-prefix ACC_5 S2B BIT_2 CT)
					(isolate-suffix ACC_6 S2B BIT_2 CT)
					(power P BIT_1 CT)
					(if-eq CT LLARGEMO
				    	(begin
				      		(eq T1_NEW (+ T1 (- ACC_3 ACC_1)))
				      		(eq T2_NEW (+ (* ACC_4 P) ACC_5))
				      		(eq T3_NEW (+ T3 (* (- ACC_6 ACC_2) P)))))))
;======================================================================



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                              ;;
;;              4.9 [3 => 2 Full]               ;;
;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;[3 => 2 Full]
(defun (three-to-two-full
		S1 S2 S3 T1 T2
		S1B S2B S3B
		BIT_1 BIT_2 P SM
		ACC_1 ACC_2 ACC_3 ACC_4 CT)
;______________________________________________________________________

				(begin
					(plateau BIT_1 SM CT)
					(plateau BIT_2 (- LLARGE SM) CT)
					(isolate-suffix ACC_1 S1B BIT_1 CT)
					(isolate-prefix ACC_2 S2B BIT_1 CT)
					(isolate-suffix ACC_3 S2B BIT_1 CT)
					(isolate-prefix ACC_4 S3B BIT_1 CT)
					(power P BIT_2 CT)
					(if-eq CT LLARGEMO
						(begin
							(eq T1 (+ (* P ACC_1) ACC_2))
							(eq T2 (+ (* P ACC_3) ACC_4))))))
;======================================================================