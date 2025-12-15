# SELFDESTRUCT related testing

## The complexity

SELFDESTRUCT's are complex. They interact with
- **balance**
- **storage**
- **code**
- they are REVERT-sensitive
- they behave differently in a CALL/CALLCODE/DELEGATECALL/STATICCALL execution context
- they lead to the (temporary) deployment of empty bytecode if performed in a deployment context

What further complicates their analysis is that they only take effect at transaction end.
Thus inside of the transaction an account MARKED_FOR_SELFDESTRUCT will
- remain callable with the same code
- its storage can still be interacted with
- it can regain balance (which will be wiped in the end)
- it can do more deployments ... which will be unaffected by the SD
Thus multi block tests will be required.

They also allow one to (attempt) to re-CREATE(2) at the same address.

There is the classical example of how to produce a CREATE address collision:
- accountA CREATE2's accountB with init code ic
- accountB CREATE's accountC when its nonce is n
- accountB SELFDESTRUCT's
- accountA re-CREATE2's accountB with the same init code ic (and same deployed code)
- accountB re-CREATE's accountC, using the same nonce n ...
- ... which leads to a collision of addresses with CREATE :D

## The tests

- [ ] repeated SELFDESTRUCT's
  - [ ] same SMC sd's several times in same transaction
- [ ] sd'ing + getting a value transfer before deletion
- [ ] sd'ing + storage all the time repeatedly afterwards
- [ ] sd'ing + reverting the sd
- [ ] sd'ing + [y/n] reverting the sd + sd'ing again + [y/n] reverting the sd
  - [ ] while deploying new accounts
- [ ] sd'ing in the root of a message call transaction
- [ ] sd'ing in the root of a deployment transaction
  - **Note.** This leads to a temporary deployment of empty bytecode that immediately gets wiped.
- [ ] sd'ing and redeploying