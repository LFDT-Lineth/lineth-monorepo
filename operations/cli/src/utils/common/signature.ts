import axios from "axios";
import { Agent } from "https";
import { err, ok, Result } from "neverthrow";
import { Hex, serializeTransaction, TransactionSerializable } from "viem";

export async function getWeb3SignerSignature(
  web3SignerUrl: string,
  web3SignerPublicKey: string,
  transaction: TransactionSerializable,
  agent?: Agent,
): Promise<Result<Hex, Error>> {
  try {
    const { data } = await axios.post(
      `${web3SignerUrl}/api/v1/eth1/sign/${web3SignerPublicKey}`,
      {
        data: serializeTransaction(transaction),
      },
      { httpsAgent: agent },
    );
    return ok(data);
  } catch (error) {
    return err(error as Error);
  }
}
