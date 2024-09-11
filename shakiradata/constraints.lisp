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
;; Shorthands
(defun    (is-keccak)    (force-bool (+ IS_KECCAK_DATA
                                        IS_KECCAK_RESULT
                                        ;; IS_SHA2_DATA     
                                        ;; IS_SHA2_RESULT   
                                        ;; IS_RIPEMD_DATA     
                                        ;; IS_RIPEMD_RESULT   
                                        )))

(defun    (is-sha2)    (force-bool (+  ;; IS_KECCAK_DATA
                                     ;; IS_KECCAK_RESULT 
                                     IS_SHA2_DATA
                                     IS_SHA2_RESULT
                                     ;; IS_RIPEMD_DATA     
                                     ;; IS_RIPEMD_RESULT   
                                     )))

(defun    (is-ripemd)    (force-bool (+  ;; IS_KECCAK_DATA
                                       ;; IS_KECCAK_RESULT 
                                       ;; IS_SHA2_DATA     
                                       ;; IS_SHA2_RESULT   
                                       IS_RIPEMD_DATA
                                       IS_RIPEMD_RESULT
                                       )))

(defun    (is-data)    (force-bool (+ IS_KECCAK_DATA
                                      ;; IS_KECCAK_RESULT 
                                      IS_SHA2_DATA
                                      ;; IS_SHA2_RESULT   
                                      IS_RIPEMD_DATA
                                      ;; IS_RIPEMD_RESULT   
                                      )))

(defun    (is-result)    (force-bool (+  ;; IS_KECCAK_DATA
                                       IS_KECCAK_RESULT
                                       ;; IS_SHA2_DATA     
                                       IS_SHA2_RESULT
                                       ;; IS_RIPEMD_DATA     
                                       IS_RIPEMD_RESULT
                                       )))

(defun (is-first-data-row)
  (force-bool (* (is-data)
                 (- 1 (prev (is-data))))))

(defun (flag-sum)
  (force-bool (+ (is-keccak) (is-sha2) (is-ripemd))))

(defun    (phase-sum)    (+    (* PHASE_KECCAK_DATA     IS_KECCAK_DATA)
                               (* PHASE_KECCAK_RESULT   IS_KECCAK_RESULT)
                               (* PHASE_SHA2_DATA       IS_SHA2_DATA)
                               (* PHASE_SHA2_RESULT     IS_SHA2_RESULT)
                               (* PHASE_RIPEMD_DATA     IS_RIPEMD_DATA)
                               (* PHASE_RIPEMD_RESULT   IS_RIPEMD_RESULT)
                               ))

(defun    (stamp-increment)    (force-bool    (*    (-    1    (is-data))
                                                    (next      (is-data)))))

(defun (index-reset-bit)
  (force-bool (+ (* (- 1 (is-data)) (next (is-data)))
                 (* (- 1 (is-result)) (next (is-result))))))

(defun    (legal-transitions-bit)    (force-bool    (+    (* IS_KECCAK_DATA      (next (is-keccak)))
                                                          (* IS_SHA2_DATA        (next (is-sha2)))
                                                          (* IS_RIPEMD_DATA      (next (is-ripemd)))
                                                          ;;
                                                          (* IS_KECCAK_RESULT    (next IS_KECCAK_RESULT))
                                                          (* IS_SHA2_RESULT      (next IS_SHA2_RESULT))
                                                          (* IS_RIPEMD_RESULT    (next IS_RIPEMD_RESULT))
                                                          ;;
                                                          (* (is-result)         (next (is-data))))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                         ;;
;;    X.3.3 Constancies    ;;
;;                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (ripsha-stamp-constancy X)
  (if-not-zero (- SHAKIRA_STAMP
                  (+ 1 (prev SHAKIRA_STAMP)))
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
         (if-zero SHAKIRA_STAMP
                  (eq! (flag-sum) 0)
                  (eq! (flag-sum) 1))
         (eq! PHASE (phase-sum))))

(defconstraint set-total-size-for-result (:guard (is-result))
  (eq! TOTAL_SIZE WORD_SIZE))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.3.5 Heartbeat    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint initial-vanishing-constraints (:domain {0})
  (vanishes! SHAKIRA_STAMP))

;; please do not touch: ""

(defconstraint padding-constraints ()
  (if-zero (flag-sum)
           (begin (vanishes! ID)
                  (vanishes! INDEX)
                  (debug (vanishes! INDEX_MAX)))))

(defconstraint stamp-increments ()
               (will-eq!    SHAKIRA_STAMP
                            (+ SHAKIRA_STAMP (stamp-increment))))

(defconstraint index-resetting ()
  (if-eq (index-reset-bit) 1
         (vanishes! (next INDEX))))

(defconstraint evolution-constraints ()
  (if-not-zero SHAKIRA_STAMP
               (begin (eq! (legal-transitions-bit) 1)
                      (if-eq-else INDEX INDEX_MAX
                                  ;; INDEX = INDEX_MAX case
                                  (eq! (index-reset-bit) 1)
                                  ;; INDEX ≠ INDEX_MAX case
                                  (will-eq! INDEX (+ 1 INDEX))))))

(defconstraint fixed-length-index-max-constraints (:guard (is-result))
  (eq! INDEX_MAX INDEX_MAX_RESULT))

;(defconstraint finalization (:domain {-1})  ;;debug end constraint
;  (if-not-zero SHAKIRA_STAMP
;               (begin (eq! INDEX INDEX_MAX)
;                      (eq! (is-result) 1))))
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    X.3.6 nBYTES accumulation    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint initializing-nBYTES_ACC ()
  (if-not-zero (remained-constant! SHAKIRA_STAMP)
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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;;    1.3.8 SELECTOR_KECCAK_RES    ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint set-esult-selector ()
  (begin (eq! SELECTOR_KECCAK_RES_HI
              (* IS_KECCAK_RESULT (- 1 INDEX)))
         (eq! SELECTOR_SHA2_RES_HI
              (* IS_SHA2_RESULT (- 1 INDEX)))
         (eq! SELECTOR_RIPEMD_RES_HI
              (* IS_RIPEMD_RESULT (- 1 INDEX)))))


