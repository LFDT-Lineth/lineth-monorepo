import { ClaimOnL1Parameters, ClaimOnL1ReturnType } from "../actions/claimOnL1";
import { ClaimOnL2Parameters, ClaimOnL2ReturnType } from "../actions/claimOnL2";
import { DepositParameters, DepositReturnType } from "../actions/deposit";
import { WithdrawParameters, WithdrawReturnType } from "../actions/withdraw";

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface WalletClient {}

export interface L1WalletClient extends WalletClient {
  deposit<TClient, TUnit = bigint, TType = string>(
    args: DepositParameters<TClient, TUnit, TType>,
  ): Promise<DepositReturnType>;
  claimOnL1<TUnit = bigint, TType = string>(args: ClaimOnL1Parameters<TUnit, TType>): Promise<ClaimOnL1ReturnType>;
}
export interface L2WalletClient extends WalletClient {
  withdraw<TUnit = bigint, TType = string>(args: WithdrawParameters<TUnit, TType>): Promise<WithdrawReturnType>;
  claimOnL2<TUnit = bigint, TType = string>(args: ClaimOnL2Parameters<TUnit, TType>): Promise<ClaimOnL2ReturnType>;
}
