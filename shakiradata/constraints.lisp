(module shakiradata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                        ;;;;
;;;;    X.3 Generalities    ;;;;
;;;;                        ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.3.1 Binary constraints    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; (defconstraint binarities ()
;;                (begin
;;                  (is-binary IS_KECCAK_DATA   )
;;                  (is-binary IS_KECCAK_RESULT )
;;                  (is-binary IS_SHA2_DATA     )
;;                  (is-binary IS_SHA2_RESULT   )
;;                  (is-binary IS_RIPEMD_DATA     )
;;                  (is-binary IS_RIPEMD_RESULT   )
;;                  (is-binary IS_EXTRA         )))
;; Shorthands
(defun (is-keccak)
  (force-bool (+ IS_KECCAK_DATA
                 IS_KECCAK_RESULT
                 ;; IS_SHA2_DATA     
                 ;; IS_SHA2_RESULT   
                 ;; IS_RIPEMD_DATA     
                 ;; IS_RIPEMD_RESULT   
                 ;; IS_EXTRA         
                 )))

(defun (is-sha2)
  (force-bool (+  ;; IS_KECCAK_DATA
                 ;; IS_KECCAK_RESULT 
                 IS_SHA2_DATA
                 IS_SHA2_RESULT
                 ;; IS_RIPEMD_DATA     
                 ;; IS_RIPEMD_RESULT   
                 ;; IS_EXTRA         
                 )))

(defun (is-ripemd)
  (force-bool (+  ;; IS_KECCAK_DATA
                 ;; IS_KECCAK_RESULT 
                 ;; IS_SHA2_DATA     
                 ;; IS_SHA2_RESULT   
                 IS_RIPEMD_DATA
                 IS_RIPEMD_RESULT
                 ;; IS_EXTRA         
                 )))

(defun (is-data)
  (force-bool (+ IS_KECCAK_DATA
                 ;; IS_KECCAK_RESULT 
                 IS_SHA2_DATA
                 ;; IS_SHA2_RESULT   
                 IS_RIPEMD_DATA
                 ;; IS_RIPEMD_RESULT   
                 ;; IS_EXTRA         
                 )))

(defun (is-result)
  (force-bool (+  ;; IS_KECCAK_DATA
                 IS_KECCAK_RESULT
                 ;; IS_SHA2_DATA     
                 IS_SHA2_RESULT
                 ;; IS_RIPEMD_DATA     
                 IS_RIPEMD_RESULT
                 ;; IS_EXTRA         
                 )))

(defun (flag-sum)
  (force-bool (+ (is-keccak) (is-sha2) (is-ripemd) IS_EXTRA)))

(defun (phase-sum)
  (+ (* PHASE_KECCAK_DATA IS_KECCAK_DATA)
     (* PHASE_KECCAK_RESULT IS_KECCAK_RESULT)
     (* PHASE_SHA2_DATA IS_SHA2_DATA)
     (* PHASE_SHA2_RESULT IS_SHA2_RESULT)
     (* PHASE_RIPEMD_DATA IS_RIPEMD_DATA)
     (* PHASE_RIPEMD_RESULT IS_RIPEMD_RESULT)
     ;; IS_EXTRA         
     ))

(defun (stamp-increment)
  (force-bool (* (- 1 (is-data)) (next (is-data)))))

(defun (index-reset-bit)
  (force-bool (+ (* (- 1 (is-data)) (next (is-data)))
                 (* (- 1 (is-result)) (next (is-result)))
                 (* (- 1 IS_EXTRA) (next IS_EXTRA)))))

(defun (legal-transitions-bit)
  (force-bool (+ (* IS_KECCAK_DATA (next (is-keccak)))
                 (* IS_SHA2_DATA (next (is-sha2)))
                 (* IS_RIPEMD_DATA (next (is-ripemd)))
                 ;;
                 (* IS_KECCAK_RESULT (next IS_KECCAK_RESULT))
                 (* IS_SHA2_RESULT (next IS_SHA2_RESULT))
                 (* IS_RIPEMD_RESULT (next IS_RIPEMD_RESULT))
                 ;;
                 (* (is-result) (next IS_EXTRA))
                 (* IS_EXTRA
                    (next (+ IS_EXTRA (is-data)))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    X.3.3 Constancies    ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ripsha-stamp-constancy X)
  (if-not-zero (- RIPSHA_STAMP
                  (+ 1 (prev RIPSHA_STAMP)))
               (remained-constant! X)))

(defconstraint stamp-constancies ()
  (ripsha-stamp-constancy ID))

(defconstraint index-constancies ()
  (begin (counter-constancy INDEX (phase-sum))
         (counter-constancy INDEX INDEX_MAX)
         (counter-constancy INDEX TOTAL_SIZE)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    X.3.4 Decoding constraints    ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint decoding-constraints ()
  (begin (debug (is-binary (flag-sum)))
         (if-zero RIPSHA_STAMP
                  (eq! (flag-sum) 0)
                  (eq! (flag-sum) 1))
         (eq! PHASE (phase-sum))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.3.5 Heartbeat    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint initial-vanishing-constraints (:domain {0})
  (vanishes! RIPSHA_STAMP))

(defconstraint padding-constraints ()
  (if-zero (flag-sum)
           (begin (vanishes! ID)
                  (vanishes! INDEX)
                  (debug (vanishes! INDEX_MAX)))))

(defconstraint stamp-increments ()
  (will-eq! RIPSHA_STAMP (+ RIPSHA_STAMP (stamp-increment))))

(defconstraint index-resetting ()
  (if-eq (index-reset-bit) 1
         (vanishes! (next INDEX))))

(defconstraint evolution-constraints ()
  (if-not-zero RIPSHA_STAMP
               (begin (eq! (legal-transitions-bit) 1)
                      (if-eq-else INDEX INDEX_MAX
                                  ;; INDEX = INDEX_MAX case
                                  (eq! (index-reset-bit) 1)
                                  ;; INDEX ≠ INDEX_MAX case
                                  (will-eq! INDEX (+ 1 INDEX))))))

(defconstraint fixed-length-index-max-constraints ()
  (if (force-bool (+ (is-result) IS_EXTRA))
      (eq! INDEX_MAX (+ IS_KECCAK_RESULT IS_SHA2_RESULT IS_RIPEMD_RESULT IS_EXTRA IS_EXTRA))))

(defconstraint small-ID-increments ()
  (if-not-zero (will-remain-constant! RIPSHA_STAMP)
               (will-eq! ID
                         (+ 1
                            ID
                            (+ (* (^ 256 3) (shift DELTA_BYTE 1))
                               (* (^ 256 2) (shift DELTA_BYTE 2))
                               (* (^ 256 1) (shift DELTA_BYTE 3))
                               (* (^ 256 0) (shift DELTA_BYTE 4)))))))

(defconstraint smallness-of-last-nBYTES ()
  (if-zero (prev IS_EXTRA)
           (if-not-zero IS_EXTRA
                        (begin (eq! (shift DELTA_BYTE 1)
                                    (- (shift nBYTES -3) 1))
                               (eq! (shift DELTA_BYTE 2)
                                    (- (+ 240 (shift nBYTES -3))
                                       1))))))

(defconstraint finalization (:domain {-1})
  (if-not-zero RIPSHA_STAMP
               (begin (eq! INDEX INDEX_MAX)
                      (eq! IS_EXTRA 1))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    X.3.6 nBYTES accumulation    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint initializing-nBYTES_ACC ()
  (if-not-zero (remained-constant! RIPSHA_STAMP)
               (eq! nBYTES nBYTES_ACC)))

(defconstraint updating-nBYTES_ACC-and-ensuring-full-limbs ()
  (if-eq (prev (is-data)) 1
         (if-eq (is-data) 1
                (begin (eq! (prev nBYTES) LLARGE)
                       (eq! nBYTES_ACC
                            (+ (prev nBYTES_ACC) nBYTES))))))

(defconstraint achieving-total-size ()
  (if-eq (prev (is-data)) 1
         (if-eq (is-result) 1
                (prev (eq! nBYTES_ACC TOTAL_SIZE)))))

