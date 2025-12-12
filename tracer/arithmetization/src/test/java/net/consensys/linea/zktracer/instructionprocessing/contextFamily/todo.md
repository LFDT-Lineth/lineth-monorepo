# Testing the CONTEXT family

- [ ] in the root context
  - [x] of a message call transaction
  - [ ] of a deployment transaction
- [ ] in the child context
  - [x] after a CALL-type instruction
    - [x] CALL
    - [x] CALLCODE
    - [ ] DELEGATECALL
    - [x] STATICCALL
  - [ ] of a CREATE-type instruction
    - [ ] CREATE
    - [ ] CREATE2
