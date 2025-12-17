(module rlptxn)

;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;;    Shorthands I    ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (is-access-list-prefix)    (* (prev TXN)     IS_ACCESS_LIST))
(defun    (is-access-list-data)      (force-bin (+   IS_PREFIX_OF_ACCESS_LIST_ITEM
                                                     IS_PREFIX_OF_STORAGE_KEY_LIST
                                                     IS_ACCESS_LIST_ADDRESS
                                                     IS_ACCESS_LIST_STORAGE_KEY
                                                     )))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    Shorthands II    ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;


(defun (rlptxn---access-list---AL-RLP-length-countdown)         cmp/AUX_1)
(defun (rlptxn---access-list---AL-item-RLP-length-countdown)    cmp/AUX_2)
(defun (rlptxn---access-list---access-list-item-countdown)      cmp/AUX_CCC_1)
(defun (rlptxn---access-list---storage-key-countdown)           cmp/AUX_CCC_2)
(defun (rlptxn---access-list---storage-key-list-countdown)      cmp/AUX_CCC_3) ;; ""

;; ;; shorthand defined outside of the module to be accessible in lookups
;; (defun (rlptxn---access-list---address-hi)          rlptxn.cmp/AUX_CCC_4)
;; (defun (rlptxn---access-list---address-lo)          rlptxn.cmp/AUX_CCC_5)

(defun (storage-stuff)                   (force-bin (+ IS_PREFIX_OF_STORAGE_KEY_LIST IS_ACCESS_LIST_STORAGE_KEY)))
(defun (not-storage-stuff)               (force-bin (- 1 (storage-stuff))))
(defun (end-of-tuple-or-end-of-phase)    (* (storage-stuff) (next (not-storage-stuff))))
