import { OnChainMessageStatus } from "@consensys/linea-sdk";
import { formatBalance, formatTimestamp } from "@/utils/format";
import { NETWORK_ID_TO_NAME } from "@/utils/constants";
import { MessageWithStatus } from "@/hooks";
import { TransactionHistory } from "@/models/history";
import TransactionClaimButton from "./TransactionClaimButton";
import TransactionDetailRow from "./TransactionDetailsRow";
import BlockExplorerLink from "./BlockExplorerLink";
import { useTransactionReceipt } from "wagmi";
import { formatEther, zeroAddress } from "viem";
import useTokenPrices from "@/hooks/useTokenPrices";

type TransactionDetailsModalProps = {
  transaction: TransactionHistory;
  message: MessageWithStatus;
  handleClose: () => void;
};

const TransactionDetailsModal: React.FC<TransactionDetailsModalProps> = ({ transaction, message, handleClose }) => {
  const { data: tokenPrices } = useTokenPrices([zeroAddress], transaction.fromChain.id);
  const { data: initialTransactionReceipt } = useTransactionReceipt({
    hash: transaction.transactionHash,
    chainId: transaction.fromChain.id,
    query: {
      enabled: message.status === OnChainMessageStatus.CLAIMED,
    },
  });

  const { data: claimingTransactionReceipt } = useTransactionReceipt({
    hash: message.claimingTransactionHash as `0x${string}`,
    chainId: transaction.toChain.id,
    query: {
      enabled: !!message.claimingTransactionHash && message.status === OnChainMessageStatus.CLAIMED,
    },
  });

  const initialTransactionFee =
    initialTransactionReceipt?.gasUsed && initialTransactionReceipt?.effectiveGasPrice
      ? initialTransactionReceipt.gasUsed * initialTransactionReceipt.effectiveGasPrice
      : 0n;

  const claimingTransactionFee =
    claimingTransactionReceipt?.gasUsed && claimingTransactionReceipt?.effectiveGasPrice
      ? claimingTransactionReceipt.gasUsed * claimingTransactionReceipt.effectiveGasPrice
      : 0n;

  return (
    <div className="flex flex-col gap-8 px-4">
      <h2 className="text-xl">Transaction details</h2>
      <div className="space-y-2">
        <TransactionDetailRow
          label="Date & Time"
          value={formatTimestamp(Number(transaction.timestamp), "h:mma d MMMM yyyy")}
        />

        <TransactionDetailRow
          label={`${NETWORK_ID_TO_NAME[transaction.fromChain.id]} Tx Hash`}
          value={
            <BlockExplorerLink
              blockExplorer={transaction.fromChain.blockExplorers?.default.url}
              transactionHash={transaction.transactionHash}
            />
          }
        />

        <TransactionDetailRow
          label={`${NETWORK_ID_TO_NAME[transaction.toChain.id]} Tx Hash`}
          value={
            <BlockExplorerLink
              blockExplorer={transaction.toChain.blockExplorers?.default.url}
              transactionHash={message.claimingTransactionHash}
            />
          }
        />
        {message.status === OnChainMessageStatus.CLAIMED && (
          <>
            <TransactionDetailRow
              label={`${NETWORK_ID_TO_NAME[transaction.fromChain.id]} fees`}
              value={
                <div>
                  {transaction.toChain.id === 1 ? (
                    <>
                      {tokenPrices[zeroAddress]?.usd ? (
                        <span>
                          {(tokenPrices[zeroAddress].usd * Number(formatEther(initialTransactionFee))).toLocaleString(
                            "en-US",
                            {
                              style: "currency",
                              currency: "USD",
                              maximumFractionDigits: 4,
                            },
                          )}
                        </span>
                      ) : (
                        `${formatBalance(formatEther(initialTransactionFee), 8)} ETH`
                      )}
                    </>
                  ) : (
                    `${formatBalance(formatEther(initialTransactionFee), 8)} ETH`
                  )}
                </div>
              }
            />
            <TransactionDetailRow
              label={`${NETWORK_ID_TO_NAME[transaction.toChain.id]} fees`}
              value={
                <div>
                  {transaction.toChain.id === 1 ? (
                    <>
                      {tokenPrices[zeroAddress]?.usd ? (
                        <span>
                          {(tokenPrices[zeroAddress].usd * Number(formatEther(claimingTransactionFee))).toLocaleString(
                            "en-US",
                            {
                              style: "currency",
                              currency: "USD",
                              maximumFractionDigits: 4,
                            },
                          )}
                        </span>
                      ) : (
                        `${formatBalance(formatEther(claimingTransactionFee), 8)} ETH`
                      )}
                    </>
                  ) : (
                    `${formatBalance(formatEther(claimingTransactionFee), 8)} ETH`
                  )}
                </div>
              }
            />
          </>
        )}
      </div>
      {message.status === OnChainMessageStatus.CLAIMABLE && (
        <TransactionClaimButton
          key={message.messageHash}
          message={message}
          transaction={transaction}
          handleClose={handleClose}
        />
      )}
    </div>
  );
};

export default TransactionDetailsModal;
