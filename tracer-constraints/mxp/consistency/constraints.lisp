(module mxp)

;; mscp_SCENARIO
;; mscp_CN
;; mscp_STAMP
;; mscp_C_MEM
;; mscp_C_MEM_NEW
;; mscp_WORDS
;; mscp_WORDS_NEW

(defconstraint  consistency---initialization---first-scenario-row ()
                (if-zero  (prev  mscp_SCENARIO)
                          (if-not-zero   mscp_SCENARIO
                                         (begin
                                           (vanishes!  mscp_WORDS)
                                           (vanishes!  mscp_C_MEM)
                                           ))))


(defconstraint  consistency---initialization---first-encounter-with-context ()
                (if-not-zero  (prev  mscp_SCENARIO)
                              (if-not-zero   mscp_SCENARIO
                                             (if-not   (remained-constant!   mscp_CN)
                                                            (begin
                                                              (vanishes!  mscp_WORDS)
                                                              (vanishes!  mscp_C_MEM)
                                                              )))))


(defconstraint  consistency---linking-constraints ()
                (if-not-zero  (prev  mscp_SCENARIO)
                              (if-not-zero   mscp_SCENARIO
                                             (if   (remained-constant!   mscp_CN)
                                                        (begin
                                                          (eq!  mscp_WORDS  (prev  mscp_WORDS_NEW))
                                                          (eq!  mscp_C_MEM  (prev  mscp_C_MEM_NEW))
                                                          )))))
