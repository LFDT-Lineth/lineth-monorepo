(module blockdata)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;  2.X INST constraints  ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   inst---unconditionally-setting-the-INST-column ()
		 (eq! INST   (inst-sum)))
