(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                 ;;;;
;;;;    X.Y CREATE   ;;;;
;;;;                 ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                         ;;
;;    X.Y.1 Introduction                                   ;;
;;    X.Y.2 Supported instructions and flags               ;;
;;    X.Y.3 Highlevel processing diagram                   ;;
;;    X.Y.4 Forward and backward setting of scenario row   ;;
;;                                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    create-instruction---forward-scenario-setting   ()
                  (if-not-zero   PEEK_AT_STACK
                                 (if-not-zero   stack/CREATE_FLAG
                                                (if-not-zero   (-   1   CT_TLI)
                                                               (if-not-zero   (-   1   stack/SUX    stack/SOX)
                                                                              (begin   (eq!   (shift   PEEK_AT_SCENARIO                      2)   1)
                                                                                       (eq!   (shift   (scenario-shorthand---CREATE---sum)   2)   1)))))))

(defconstraint   create-instruction---backward-setting-CREATE-instruction   ()
                 (if-not-zero   PEEK_AT_SCENARIO
                                (if-not-zero   (scenario-shorthand---CREATE---sum)
                                               (begin    (eq!    (shift    PEEK_AT_STACK       CREATE_first_stack_row___row_offset)    1)
                                                         (eq!    (shift    stack/CREATE_FLAG   CREATE_first_stack_row___row_offset)    1)
                                                         (eq!    (shift    CT_TLI              CREATE_first_stack_row___row_offset)    0)
                                                         (eq!    (shift    stack/SUX           CREATE_first_stack_row___row_offset)    0)))))
