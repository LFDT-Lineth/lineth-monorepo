(module rlptxn)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;;    X.Y.Z.T AL_item_RLP_length_coundown constraints    ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    rlptxn---access-list---AL-item-RLP-length-countdown---update ()
                  (if-not-zero    (force-bin   (+ IS_ACCESS_LIST_ADDRESS
                                                  IS_PREFIX_OF_STORAGE_KEY_LIST
                                                  IS_ACCESS_LIST_STORAGE_KEY))
                                  (did-dec!    (rlptxn---access-list---AL-item-RLP-length-countdown)
                                               (* LC cmp/LIMB_SIZE))))

(defconstraint    rlptxn---access-list---AL-item-RLP-length-countdown---finalization ()
                  (if-not-zero    (end-of-tuple-or-end-of-phase)
                                  (vanishes! (rlptxn---access-list---AL-item-RLP-length-countdown))))
