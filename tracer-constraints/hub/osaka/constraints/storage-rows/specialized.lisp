(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.1 Storage-rows   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    X.1.5 Specialized constraints   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (storage-reading kappa)
  (begin (eq! (shift storage/VALUE_CURR_HI kappa) (shift storage/VALUE_NEXT_HI kappa))
         (eq! (shift storage/VALUE_CURR_LO kappa) (shift storage/VALUE_NEXT_LO kappa))))

(defun (storage-turn-on-warmth kappa)
  (eq! (shift storage/WARMTH_NEW kappa) 1))

(defun (storage-same-warmth kappa)
  (eq! (shift storage/WARMTH_NEW kappa)
       (shift storage/WARMTH     kappa)))

(defun (storage-same-slot kappa)
  (begin (remained-constant! (shift storage/ADDRESS_HI        kappa) )
         (remained-constant! (shift storage/ADDRESS_LO        kappa) )
         (remained-constant! (shift storage/STORAGE_KEY_LO    kappa) )
         (remained-constant! (shift storage/STORAGE_KEY_HI    kappa) )
         (remained-constant! (shift storage/DEPLOYMENT_NUMBER kappa) )))

(defun (undo-storage-warmth-update kappa)
  (begin (shift (was-eq! storage/WARMTH_NEW  storage/WARMTH    ) kappa)
         (shift (was-eq! storage/WARMTH      storage/WARMTH_NEW) kappa)))

(defun (undo-storage-value-update kappa)
  (begin (shift (was-eq! storage/VALUE_NEXT_HI storage/VALUE_CURR_HI) kappa)
         (shift (was-eq! storage/VALUE_NEXT_LO storage/VALUE_CURR_LO) kappa)
         (shift (was-eq! storage/VALUE_CURR_HI storage/VALUE_NEXT_HI) kappa)
         (shift (was-eq! storage/VALUE_CURR_LO storage/VALUE_NEXT_LO) kappa)))

(defun (undo-storage-warmth-and-value-update kappa)
  (begin (undo-storage-warmth-update kappa)
         (undo-storage-value-update  kappa)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.1.5 Binary columns for gas cost   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-storage-binary-flag-VALUE_ORIG_IS_ZERO (:perspective storage)
               (begin
                 (if-not-zero VALUE_ORIG_HI
                              (vanishes! VALUE_ORIG_IS_ZERO)
                              (if-not-zero VALUE_ORIG_LO
                                           (vanishes! VALUE_ORIG_IS_ZERO)
                                           (eq!       VALUE_ORIG_IS_ZERO 1)))))

(defconstraint setting-storage-binary-flag-VALUE_CURR_IS_ZERO (:perspective storage)
               (begin
                 (if-not-zero VALUE_CURR_HI
                              (vanishes! VALUE_CURR_IS_ZERO)
                              (if-not-zero VALUE_CURR_LO
                                           (vanishes! VALUE_CURR_IS_ZERO)
                                           (eq!       VALUE_CURR_IS_ZERO 1)))))

(defconstraint setting-storage-binary-flag-VALUE_NEXT_IS_ZERO (:perspective storage)
               (begin
                 (if-not-zero VALUE_NEXT_HI
                              (vanishes! VALUE_NEXT_IS_ZERO)
                              (if-not-zero VALUE_NEXT_LO
                                           (vanishes! VALUE_NEXT_IS_ZERO)
                                           (eq!       VALUE_NEXT_IS_ZERO 1)))))

(defconstraint setting-storage-binary-flag-VALUE_CURR_IS_ORIG (:perspective storage)
               (begin
                 (if-not-zero (- VALUE_CURR_HI VALUE_ORIG_HI)
                              (vanishes! VALUE_CURR_IS_ORIG)
                              (if-not-zero (- VALUE_CURR_LO VALUE_ORIG_LO)
                                           (vanishes! VALUE_CURR_IS_ORIG)
                                           (eq!       VALUE_CURR_IS_ORIG 1)))))

(defconstraint setting-storage-binary-flag-VALUE_NEXT_IS_CURR (:perspective storage)
               (begin
                 (if-not-zero (- VALUE_NEXT_HI VALUE_CURR_HI)
                              (vanishes! VALUE_NEXT_IS_CURR)
                              (if-not-zero (- VALUE_NEXT_LO VALUE_CURR_LO)
                                           (vanishes! VALUE_NEXT_IS_CURR)
                                           (eq!       VALUE_NEXT_IS_CURR 1)))))

(defconstraint setting-storage-binary-flag-VALUE_NEXT_IS_ORIG (:perspective storage)
               (begin
                 (if-not-zero (- VALUE_NEXT_HI VALUE_ORIG_HI)
                              (vanishes! VALUE_NEXT_IS_ORIG)
                              (if-not-zero (- VALUE_NEXT_LO VALUE_ORIG_LO)
                                           (vanishes! VALUE_NEXT_IS_ORIG)
                                           (eq!       VALUE_NEXT_IS_ORIG 1)))))
