import { Address } from "viem";
import { UpdateVaultDataParams } from "./contracts/ILazyOracle.js";

export interface ILidoAccountingReportClient {
  getLatestSubmitVaultReportParams(vault: Address): Promise<UpdateVaultDataParams>;
  submitLatestVaultReport(vault: Address): Promise<void>;
}
