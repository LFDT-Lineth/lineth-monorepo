(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;   X.Y.Z Delegation constraints   ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (account-same-delegation-address   relof)
  (begin
    (shift  (eq!  account/DELEGATION_ADDRESS_HI_NEW  account/DELEGATION_ADDRESS_HI )  relof)
    (shift  (eq!  account/DELEGATION_ADDRESS_LO_NEW  account/DELEGATION_ADDRESS_LO )  relof)
    ))

(defun   (account-same-delegation-number   relof)
  (shift  (eq!   account/DELEGATION_NUMBER_NEW   account/DELEGATION_NUMBER )   relof))

(defun   (account-same-delegation-status   relof)
  (shift  (eq!   account/IS_DELEGATED_NEW   account/IS_DELEGATED )   relof))

(defun   (account-set-delegation-address   relof
                                           address_hi
                                           address_lo)
  (begin   (eq!   (shift   account/DELEGATION_ADDRESS_HI_NEW   relof )   address_hi )
           (eq!   (shift   account/DELEGATION_ADDRESS_LO_NEW   relof )   address_lo )
           ))

(defun   (account-increment-delegation-number   relof)
  (shift   (eq!   account/DELEGATION_NUMBER_NEW   (+   account/DELEGATION_NUMBER   1))   relof))

;;-----------------------------------------------;;
;;   Check for delegation compound constraints   ;;
;;-----------------------------------------------;;

(defun   (account-check-for-delegation-in-authorization-phase  relof)
  (begin   (shift  (eq!   account/CHECK_FOR_DELEGATION       account/HAS_CODE     )  relof )
           (shift  (eq!   account/CHECK_FOR_DELEGATION_NEW   account/HAS_CODE_NEW )  relof )
           ))

(defun   (account-conditionally-check-for-delegation   relof
                                                       condition)
  (begin   (eq!   (shift   account/CHECK_FOR_DELEGATION       relof)   condition )
           (eq!   (shift   account/CHECK_FOR_DELEGATION_NEW   relof)   0         )
           ))

(defun   (account-check-for-delegation-if-account-has-code   relof)
  (begin   (shift  (eq!         account/CHECK_FOR_DELEGATION        account/HAS_CODE )  relof)
           (shift  (vanishes!   account/CHECK_FOR_DELEGATION_NEW                     )  relof)
           ))

(defun   (account-dont-check-for-delegation   relof)
  (begin   (vanishes!   (shift  account/CHECK_FOR_DELEGATION      relof) )
           (vanishes!   (shift  account/CHECK_FOR_DELEGATION_NEW  relof) )
           ))

