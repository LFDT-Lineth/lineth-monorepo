import "hardhat/types/global-options";

declare module "hardhat/types/global-options" {
  interface GlobalOptions {
    /** `--signer-ui` or `HARDHAT_SIGNER_UI=true`: sign in the browser wallet inside `withSignerUi` scopes. */
    signerUi: boolean;
  }
}
