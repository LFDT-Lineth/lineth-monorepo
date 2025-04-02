(module logdata)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.1 Heartbeat    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;
(defconstraint first-row (:domain {0})
  (vanishes! ABS_LOG_NUM))

(defconstraint forced-vanishing ()
  (if-zero ABS_LOG_NUM
           (begin (vanishes! LOGS_DATA)
                  (vanishes! SIZE_TOTAL)
                  (vanishes! SIZE_ACC)
                  (vanishes! SIZE_LIMB)
                  (vanishes! LIMB)
                  (vanishes! INDEX))))

(defconstraint number-increments ()
  (or! (remained-constant! ABS_LOG_NUM) (did-inc! ABS_LOG_NUM 1)))

(defconstraint index-reset ()
  (if-not (remained-constant! ABS_LOG_NUM)
          (vanishes! INDEX)))

(defconstraint log-logs-no-data (:guard ABS_LOG_NUM)
  (if-zero LOGS_DATA
           (begin (vanishes! SIZE_TOTAL)
                  (vanishes! SIZE_ACC)
                  (vanishes! SIZE_LIMB)
                  (vanishes! LIMB)
                  (did-inc! ABS_LOG_NUM 1))))

(defconstraint log-logs-data (:guard LOGS_DATA)
  (begin (if-not (remained-constant! ABS_LOG_NUM)
                 (eq! SIZE_ACC SIZE_LIMB))
         (if-not (did-inc! ABS_LOG_NUM 1)
                 (begin (eq! SIZE_ACC
                                  (+ (prev SIZE_ACC) SIZE_LIMB))
                             (debug (eq! SIZE_LIMB LLARGE))
                             (did-inc! INDEX 1)))
         (if-not (will-remain-constant! ABS_LOG_NUM)
                 (begin (eq! SIZE_TOTAL SIZE_ACC)
                        (vanishes! (next INDEX))))
         (debug       (if-eq       SIZE_ACC   SIZE_TOTAL
                      (will-inc!    ABS_LOG_NUM   1)))))

(defconstraint final-row (:domain {-1} :guard ABS_LOG_NUM)
  (begin (eq! ABS_LOG_NUM ABS_LOG_NUM_MAX)
         (if-eq LOGS_DATA 1 (eq! SIZE_ACC SIZE_TOTAL))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    2.2 Constancies    ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (conflation-constancy   X)
  (if-not-zero   ABS_LOG_NUM
                 (will-remain-constant!   X)))

(defconstraint   conflation-constancy-of-ABS_LOG_NUM_MAX   ()
                 (conflation-constancy   ABS_LOG_NUM_MAX))

(defun (log-constancy X)
  (if-not   (did-inc!           ABS_LOG_NUM 1)
            (remained-constant! X)))

(defconstraint log-constancies ()
  (begin (log-constancy SIZE_TOTAL)
         (debug (log-constancy LOGS_DATA))))

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    2.3 LOGS_DATA    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint log-logs-data-definition ()
  (if-zero SIZE_TOTAL
           (vanishes! LOGS_DATA)
           (eq!       LOGS_DATA 1)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    2.4 Range check for SIZE_LIMB    ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (normalized-SIZE_LIMB)
  (* LOGS_DATA (- SIZE_LIMB 1)))

;; this constraint enforces in particular that the final value of SIZE_LIMB is in the range [1 , 16]
(definrange    (normalized-SIZE_LIMB)   16)
