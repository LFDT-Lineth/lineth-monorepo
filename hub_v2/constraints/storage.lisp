(module hub_v2)

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;  7.1 binary columns for gas cost ;; 
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint sto_binarity (:perspective storage)
 (begin
  (is-binary VAL_ORIG_IS_ZERO)
  (is-binary VAL_CURR_IS_ORIG)
  (is-binary VAL_CURR_IS_ZERO)
  (is-binary VAL_NEXT_IS_CURR)
  (is-binary VAL_NEXT_IS_ZERO)
  (is-binary VAL_NEXT_IS_ORIG)))

(defpurefun (is_hi_and_lo_null VAL_HI VAL_LO VAL_BIN)
 (if-zero VAL_HI
    (if-zero VAL_LO
        (eq! VAL_BIN 1)
        (vanishes! VAL_BIN))
    (vanishes! VAL_BIN)))

(defconstraint set_val_is_zero (:perspective storage)
 (begin
  (is_hi_and_lo_null VAL_ORIG_HI VAL_ORIG_LO VAL_ORIG_IS_ZERO)
  (is_hi_and_lo_null VAL_CURR_HI VAL_CURR_LO VAL_CURR_IS_ZERO)
  (is_hi_and_lo_null (- VAL_CURR_HI VAL_ORIG_HI) (- VAL_CURR_LO VAL_ORIG_LO) VAL_CURR_IS_ORIG)
  (is_hi_and_lo_null (- VAL_CURR_HI VAL_NEXT_HI) (- VAL_CURR_LO VAL_NEXT_LO) VAL_NEXT_IS_CURR)
  (is_hi_and_lo_null VAL_NEXT_HI VAL_NEXT_LO VAL_NEXT_IS_ZERO)
  (is_hi_and_lo_null (- VAL_NEXT_HI VAL_ORIG_HI) (- VAL_NEXT_LO VAL_ORIG_LO) VAL_NEXT_IS_ORIG)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;  7.2 Specialized constraints ;; 
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (storage_reading_h)
 (begin
  (eq! VAL_CURR_HI VAL_NEXT_HI)
  (eq! VAL_CURR_LO VAL_NEXT_LO)))

(defun (turn_on_storage_warmth_h)
 (eq! WARM_NEW 1))

(defun (undo_storage_warmth_update_v)
 (begin
  (eq! WARM (prev WARM_NEW))
  (eq! WARM_NEW (prev WARM))))

(defun (undo_storage_value_update_v)
 (begin
  (eq! VAL_NEXT_HI (prev VAL_CURR_HI))
  (eq! VAL_NEXT_LO (prev VAL_CURR_LO))))

(defun (undo_storage_warmth_and_value_update_v)
 (begin
  (undo_storage_warmth_update_v)
  (undo_storage_value_update_v)))

(defun (same_storage_slot_v)
 (begin
  (remained-constant! ADDRESS_HI)
  (remained-constant! ADDRESS_LO)
  (remained-constant! DEPLOYMENT_NUMBER)
  (remained-constant! STORAGE_KEY_HI)
  (remained-constant! STORAGE_KEY_LO)))