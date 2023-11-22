(module mmio)

;; transplants


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                           ;;
;;              4.3 RAM to RAM               ;;
;;                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint ram-to-ram ()
                (if-eq MICRO_INSTRUCTION RamToRam
                    (begin
                        (eq CN_A CN_S)
                        (eq CN_B CN_T)
                        (vanishes! CN_C)

                        (eq INDEX_A SLO)
                        (eq INDEX_B TLO)
                        (vanishes! INDEX_C)

                        (eq VAL_A_NEW VAL_A)
                        (eq VAL_B_NEW VAL_A)
                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;              4.3 Exodata to RAM               ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint exo-to-ram ()
                (if-eq MICRO_INSTRUCTION ExoToRam
                    (begin
                        (= CN_A CN_T)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A TLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)
                        (= INDEX_X SLO)

                        (= VAL_A_NEW VAL_X)

                        (vanishes! VAL_B)        ;; TODO remove
                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_B_NEW)    ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                      ;;
;;              4.4 Exodata and RAM agree               ;;
;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint ram-is-exo ()
                (if-eq MICRO_INSTRUCTION RamIsExo
                    (begin
                        (= CN_A CN_S)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)
                        (= INDEX_X TLO)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_X VAL_A)
                        (vanishes! VAL_B)        ;; TODO remove
                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_B_NEW)    ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                  ;;
;;              4.5 Killing RAM slots               ;;
;;                                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint killing-one ()
                (if-eq MICRO_INSTRUCTION KillingOne
                    (begin
                        (= CN_A CN_T)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A TLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_X)

                        (vanishes! VAL_B)        ;; TODO remove
                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_B_NEW)    ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;              4.6 RAM to Stack               ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint push-two-ram-to-stack ()
                (if-eq MICRO_INSTRUCTION PushTwoRamToStack
                    (begin
                        (= CN_A CN_S)
                        (= CN_B CN_S)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (= INDEX_B (+ SLO 1))
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_B)

                        (= VAL_HI VAL_A)
                        (= VAL_LO VAL_B)

                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove


(defconstraint push-one-ram-to-stack ()
                (if-eq MICRO_INSTRUCTION PushOneRamToStack
                    (begin
                        (= CN_A CN_S)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (= INDEX_A SLO)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_A)
                        (= VAL_B_NEW VAL_LO)

                        (= VAL_HI VAL_A)
                        (vanishes! VAL_LO)

                        (vanishes! VAL_B)        ;; TODO remove
                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_B_NEW)    ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove

(defconstraint exceptional-ram-to-stack-three-to-two-full-fast ()
                (if-eq MICRO_INSTRUCTION ExceptionalRamToStack3To2FullFast
                    (begin
                        (vanishes! CN_A)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (vanishes!  INDEX_A)
                        (vanishes!  INDEX_B)
                        (vanishes!  INDEX_C)

                        (= VAL_HI VAL_A)
                        (= VAL_LO VAL_B)

                        (vanishes! VAL_A_NEW)    ;; TODO remove
                        (vanishes! VAL_B_NEW)    ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove

                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;              4.7 Stack to RAM               ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint push-two-stack-to-ram ()
                (if-eq MICRO_INSTRUCTION PushTwoStackToRam
                    (begin
                        (= CN_A CN_T)
                        (= CN_B CN_T)
                        (vanishes! CN_C)

                        (= INDEX_A TLO)
                        (= INDEX_B (+ TLO 1))
                        (vanishes! INDEX_C)

                        (= VAL_A_NEW VAL_HI)
                        (= VAL_B_NEW VAL_LO)

                        (vanishes! VAL_C)        ;; TODO remove
                        (vanishes! VAL_C_NEW)    ;; TODO remove
                        (vanishes! ERF)          ;; TODO remove
                        (= FAST 1))))           ;; TODO remove


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                             ;;
;;              4.8 Transaction call data to RAM               ;;
;;                                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint store-X-in-A-three-required ()
                (if-eq MICRO_INSTRUCTION StoreXInAThreeRequired
                    (begin
                        (vanishes! CN_A)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (vanishes! INDEX_B)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)
                        (= INDEX_X SLO)

                        (= VAL_A VAL_X)
                        (will-remain-constant! VAL_A)
                        (will-remain-constant! VAL_B)
                        (will-remain-constant! VAL_C)

                        (vanishes! VAL_A_NEW)            ;; TODO remove
                        (vanishes! VAL_B_NEW)            ;; TODO remove
                        (vanishes! VAL_C_NEW)            ;; TODO remove
                        (will-remain-constant! VAL_A_NEW)    ;; TODO remove
                        (will-remain-constant! VAL_B_NEW)    ;; TODO remove
                        (will-remain-constant! VAL_C_NEW)    ;; TODO remove
                        (= ERF 1)                       ;; TODO remove
                        (= FAST 1))))                   ;; TODO remove

(defconstraint store-X-in-B ()
                (if-eq MICRO_INSTRUCTION StoreXInB
                    (begin
                        (vanishes! CN_A)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (vanishes! INDEX_B)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)
                        (= INDEX_X SLO)

                        (= VAL_B VAL_X)
                        (will-remain-constant! VAL_A)
                        (will-remain-constant! VAL_B)
                        (will-remain-constant! VAL_C)

                        (vanishes! VAL_A_NEW)            ;; TODO remove
                        (vanishes! VAL_B_NEW)            ;; TODO remove
                        (vanishes! VAL_C_NEW)            ;; TODO remove
                        (will-remain-constant! VAL_A_NEW)    ;; TODO remove
                        (will-remain-constant! VAL_B_NEW)    ;; TODO remove
                        (will-remain-constant! VAL_C_NEW)    ;; TODO remove
                        (= ERF 1)                       ;; TODO remove
                        (= FAST 1))))                   ;; TODO remove

(defconstraint store-X-in-C ()
                (if-eq MICRO_INSTRUCTION StoreXInC
                    (begin
                        (vanishes! CN_A)
                        (vanishes! CN_B)
                        (vanishes! CN_C)

                        (vanishes! INDEX_B)
                        (vanishes! INDEX_B)
                        (vanishes! INDEX_C)
                        (= INDEX_X SLO)

                        (= VAL_C VAL_X)
                        (will-remain-constant! VAL_A)
                        (will-remain-constant! VAL_B)
                        (will-remain-constant! VAL_C)

                        (vanishes! VAL_A_NEW)            ;; TODO remove
                        (vanishes! VAL_B_NEW)            ;; TODO remove
                        (vanishes! VAL_C_NEW)            ;; TODO remove
                        (will-remain-constant! VAL_A_NEW)    ;; TODO remove
                        (will-remain-constant! VAL_B_NEW)    ;; TODO remove
                        (will-remain-constant! VAL_C_NEW)    ;; TODO remove
                        (= ERF 1)                       ;; TODO remove
                        (= FAST 1))))                   ;; TODO remove
