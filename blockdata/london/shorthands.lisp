(module blockdata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;  2.1 Shorthands          ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (flag-sum)    (+ IS_CB
                           IS_TS
                           IS_NB
                           IS_DF
                           IS_GL
                           IS_ID
                           IS_BF))

(defun    (wght-sum)    (+ ( * 1 IS_CB)
                           ( * 2 IS_TS)
                           ( * 3 IS_NB)
                           ( * 4 IS_DF)
                           ( * 5 IS_GL)
                           ( * 6 IS_ID)
                           ( * 7 IS_BF)))

(defun    (inst-sum)    (+ (* EVM_INST_COINBASE   IS_CB)
                           (* EVM_INST_TIMESTAMP  IS_TS)
                           (* EVM_INST_NUMBER     IS_NB)
                           (* EVM_INST_DIFFICULTY IS_DF)
                           (* EVM_INST_GASLIMIT   IS_GL)
                           (* EVM_INST_CHAINID    IS_ID)
                           (* EVM_INST_BASEFEE    IS_BF)))

(defun    (ct-max-sum)    (+ (* (- nROWS_CB 1) IS_CB)
                             (* (- nROWS_TS 1) IS_TS)
                             (* (- nROWS_NB 1) IS_NB)
                             (* (- nROWS_DF 1) IS_DF)
                             (* (- nROWS_GL 1) IS_GL)
                             (* (- nROWS_ID 1) IS_ID)
                             (* (- nROWS_BF 1) IS_BF)))

(defun    (phase-entry)    (+ (* (- 1 IS_CB) (next IS_CB))
                              (* (- 1 IS_TS) (next IS_TS))
                              (* (- 1 IS_NB) (next IS_NB))
                              (* (- 1 IS_DF) (next IS_DF))
                              (* (- 1 IS_GL) (next IS_GL))
                              (* (- 1 IS_ID) (next IS_ID))
                              (* (- 1 IS_BF) (next IS_BF))))

(defun    (same-phase)     (+ (* IS_CB (next IS_CB))
                              (* IS_TS (next IS_TS))
                              (* IS_NB (next IS_NB))
                              (* IS_DF (next IS_DF))
                              (* IS_GL (next IS_GL))
                              (* IS_ID (next IS_ID))
                              (* IS_BF (next IS_BF))))

(defun    (allowable-transitions)     (+ (* IS_CB (next IS_TS))
                                         (* IS_TS (next IS_NB))
                                         (* IS_NB (next IS_DF))
                                         (* IS_DF (next IS_GL))
                                         (* IS_GL (next IS_ID))
                                         (* IS_ID (next IS_BF))
                                         (* IS_BF (next IS_CB))))

(defun  (curr-data-hi)                            DATA_HI                     )
(defun  (curr-data-lo)                            DATA_LO                     )
(defun  (prev-data-hi)                    (shift  DATA_HI  (- 0 nROWS_DEPTH)))
(defun  (prev-data-lo)                    (shift  DATA_LO  (- 0 nROWS_DEPTH)))
(defun  (isnt-first-block-in-conflation)  (shift  IOMF     (- 0 nROWS_DEPTH)))
(defun  (is-first-block-in-conflation)    (-  1  (isnt-first-block-in-conflation)))
