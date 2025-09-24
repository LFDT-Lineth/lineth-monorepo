(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                         ;;
;;    X.Y.1 Introduction                                   ;;
;;    X.Y.2 Description of the general approach            ;;
;;    X.Y.3 Supported instructions and flags               ;;
;;    X.Y.4 Highlevel processing diagram                   ;;
;;    X.Y.5 Forward and backward setting of scenario row   ;;
;;                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    call-instruction---forward-scenario-setting    ()
                  (if-not-zero    PEEK_AT_STACK
                                  (if-not-zero    stack/CALL_FLAG
                                                  (if-not-zero    (-    1    CT_TLI)
                                                                  (if-not-zero   (-   1   stack/SUX    stack/SOX)
                                                                                 (begin
                                                                                   (eq!    (shift    PEEK_AT_SCENARIO                 2)    1)
                                                                                   (eq!    (shift    (scenario-shorthand---CALL---sum)    2)    1)
                                                                                   )
                                                                                 )
                                                                  )
                                                  )
                                  )
                  )

(defconstraint   call-instruction---backward-setting-CALL-instruction   ()
                 (if-not-zero    PEEK_AT_SCENARIO
                                 (if-not-zero    (scenario-shorthand---CALL---sum)
                                                 (begin
                                                   (eq!    (shift    PEEK_AT_STACK                   CALL_1st_stack_row___row_offset)    1)
                                                   (eq!    (shift    stack/CALL_FLAG                 CALL_1st_stack_row___row_offset)    1)
                                                   (eq!    (shift    CT_TLI                          CALL_1st_stack_row___row_offset)    0)
                                                   (eq!    (shift    (+   stack/SUX    stack/SOX)    CALL_1st_stack_row___row_offset)    0)
                                                   )
                                                 )
                                 )
                 )
