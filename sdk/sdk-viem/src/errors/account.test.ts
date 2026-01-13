import { setErrorConfig } from "viem";
import { AccountNotFoundError } from "./account";

describe("Account Errors", () => {
  beforeAll(() => {
    setErrorConfig({ version: "viem@x.y.z" });
  });

  describe("AccountNotFoundError", () => {
    it("should match snapshot without docsPath", () => {
      expect(new AccountNotFoundError()).toMatchInlineSnapshot(`
    [AccountNotFoundError: Could not find an Account to execute with this Action.
    Please provide an Account with the \`account\` argument on the Action, or by supplying an \`account\` to the Client.
    
    Version: viem@x.y.z]
  `);
    });

    it("should match snapshot with docsPath", () => {
      expect(
        new AccountNotFoundError({
          docsPath: "/docs/actions/wallet/sendTransaction",
        }),
      ).toMatchInlineSnapshot(`
    [AccountNotFoundError: Could not find an Account to execute with this Action.
    Please provide an Account with the \`account\` argument on the Action, or by supplying an \`account\` to the Client.
    
    Version: viem@x.y.z]
  `);
    });
  });
});
