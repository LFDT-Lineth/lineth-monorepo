(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                         ;;;;
;;;;    X.Y Transient-rows   ;;;;
;;;;                         ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;    X.Y.Z Specialized constraints   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;; no value change on the row with relative row offset "relof"
(defun (transient-storage-reading                 relof)
  (begin   (eq! (shift transient/VALUE_CURR_HI    relof) (shift transient/VALUE_NEXT_HI    relof))
           (eq! (shift transient/VALUE_CURR_LO    relof) (shift transient/VALUE_NEXT_LO    relof))))

;; identify address and key of some
;; current   row offset ("curof") with that of some
;; reference row offset ("refof")
(defun (transient-storage-same-slot              curof                                    refof)
  (begin   (eq! (shift transient/ADDRESS_HI      curof ) (shift transient/ADDRESS_HI      refof ))
           (eq! (shift transient/ADDRESS_LO      curof ) (shift transient/ADDRESS_LO      refof ))
           (eq! (shift transient/STORAGE_KEY_LO  curof ) (shift transient/STORAGE_KEY_LO  refof ))
           (eq! (shift transient/STORAGE_KEY_HI  curof ) (shift transient/STORAGE_KEY_HI  refof ))
           ))

;; undoes the change in
;; new/curr values at relative   doing offset ("reldo") by doing the opposite switch
;; curr/new values at relative undoing offset ("reluo")
(defun (transient-storage-undoing-value-update   reluo                                      reldo)
  (begin   (eq! (shift transient/VALUE_CURR_HI   reluo )  (shift   transient/VALUE_NEXT_HI  reldo ))
           (eq! (shift transient/VALUE_CURR_LO   reluo )  (shift   transient/VALUE_NEXT_LO  reldo ))
           (eq! (shift transient/VALUE_NEXT_HI   reluo )  (shift   transient/VALUE_CURR_HI  reldo ))
           (eq! (shift transient/VALUE_NEXT_LO   reluo )  (shift   transient/VALUE_CURR_LO  reldo ))
           ))
