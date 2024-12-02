import Link from "next/link";
import { Collapse } from "@/components/ui";

export default function FaqPage() {
  return (
    <div className="m-auto min-w-min max-w-5xl">
      <h1 className="mb-6 text-4xl md:hidden">FAQ</h1>
      <div className="flex flex-col gap-5">
        <Collapse title="Which address do the funds go to?">
          <p>By default, your bridged funds are sent to the same address they were originally sent from.</p>
        </Collapse>

        <Collapse title="Can I send the funds to a different address?">
          <p>
            Yes — find the &apos;To different address&apos; button beneath the field that displays the amount of ETH or
            tokens you will receive on the target chain. You can then enter the address you want the funds to be bridged
            to. Only send funds to addresses on Ethereum Mainnet or Linea Mainnet. You may lose funds otherwise, and
            they will be irretrievable (see this{" "}
            <Link
              href="https://support.metamask.io/managing-my-tokens/moving-your-tokens/funds-sent-on-wrong-network/#non-evm-tokens-and-networks"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              MetaMask Support article
            </Link>{" "}
            for an explanation). This means you cannot bridge to Bitcoin or Solana, for example, using the Linea Bridge.
          </p>
        </Collapse>

        <Collapse title="I need to claim a transaction. What do I do?">
          <p>
            If you chose manual claiming, you need to return to the Linea Bridge and make an additional transaction. It
            typically takes around 20 minutes (two Ethereum epochs) before you can claim a deposit; withdrawals from
            Linea can take much longer, and vary in duration.
          </p>
          <p>
            To claim, head to the &apos;Transactions&apos; tab and locate the bridge transaction you want to claim; it
            should be marked as &apos;Ready to claim&apos;. Click on it, and then on the &apos;Claim&apos; button to
            initiate the transaction.
          </p>
        </Collapse>

        <Collapse title="What fees will I pay?">
          <p>
            Linea does not charge fees for using the bridge. However, you will need to pay gas fees for transactions.
            Gas fees vary according to network activity, so we cannot accurately predict how much you&apos;ll pay. The
            bulk of the cost will be for interacting with Ethereum Mainnet; Linea&apos;s gas fees are considerably
            cheaper.
          </p>
          <p>
            If you use manual claiming, you pay gas fees for the bridge transaction and then gas fees for the claim
            transaction.
          </p>
          <p>
            For automatic claiming, you pay gas fees for your initial bridge transaction and then a fee to cover the gas
            costs of the postman executing the claim on your behalf.
          </p>
        </Collapse>

        <Collapse title="Why can I only select Linea Mainnet and Ethereum Mainnet?">
          <p>
            The Linea Bridge is designed to transact between layer 1 (Ethereum Mainnet) and layer 2 (Linea Mainnet),
            rather than for bridging to any other chains.
          </p>
          <p>
            If you want to bridge to other EVM-compatible networks, consider using the{" "}
            <Link
              href="https://portfolio.metamask.io/bridge"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              MetaMask Portfolio bridge
            </Link>
            .
          </p>
        </Collapse>

        <Collapse title="Why can't I see the token I want to bridge?">
          <p>
            The tokens available to select on the Linea Bridge are sourced from a curated list defined{" "}
            <Link
              href="https://github.com/Consensys/linea-token-list/blob/main/json/linea-mainnet-token-shortlist.json"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              here
            </Link>{" "}
            and maintained by the Linea team. This practice ensures users always bridge to the correct token—rather than
            variants, preventing{" "}
            <Link
              href="https://support.linea.build/bridging/bridging-to-the-correct-erc-20-token-on-linea"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              liquidity fragmentation
            </Link>{" "}
            and mitigates the risk of funds loss.
          </p>
        </Collapse>

        <Collapse title="Why can't I see my bridged tokens in my wallet?">
          <p>
            First, check whether your funds are ready to claim. To see claimable funds, go to the “Transactions” tab of
            the Linea Bridge app.
          </p>
          <p>
            If claiming isn&apos;t the issue, head to Lineascan and see if the transaction is pending, and in the queue:
          </p>
          <ul className="list-disc pl-8">
            <li>
              <Link
                href="https://lineascan.build/txsDeposits"
                rel="noopener noreferrer"
                passHref
                target="_blank"
                className="link link-secondary"
              >
                Here
              </Link>{" "}
              for L1 -&gt; L2 transactions (deposits)
            </li>
            <li>
              <Link
                href="https://lineascan.build/txsExit"
                rel="noopener noreferrer"
                passHref
                target="_blank"
                className="link link-secondary"
              >
                Here
              </Link>{" "}
              for L2 -&gt; L1 transactions (exit transactions)
            </li>
          </ul>

          <p>
            If the transaction is still pending, just wait for it to be confirmed, and your funds will be available to
            claim or will be in your wallet (depending on the claiming method you chose). If the transaction is missing,
            or it has been confirmed and you still don&pos;t see your tokens, contact support by clicking the button in
            the bottom-left of the Linea Bridge app, or head to the{" "}
            <Link
              href="https://support.linea.build/"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              Linea help center
            </Link>
            .
          </p>
        </Collapse>

        <Collapse title="How long does the bridging take?">
          <p>This depends on the direction of your bridge:</p>
          <ul className="list-disc pl-8">
            <li>Deposit (L1 -&gt; L2): Approximately 20 minutes.</li>
            <li>
              Withdrawal (L2 -&gt; L1): Between 8 and 32 hours. The L2 transaction must be finalized on Ethereum Mainnet
              before you can claim your funds.
            </li>
          </ul>
        </Collapse>

        <Collapse title="Can I speed up my transaction?">
          <p>
            Yes, although it&apos;s not a method we recommend for beginners.{" "}
            <Link
              href="https://support.linea.build/bridging/can-i-speed-up-bridge-transfers-on-the-linea-bridge"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              View the guide on the MetaMask Help Center
            </Link>
            .
          </p>
          <p>
            Note that this only speeds up your submission of the bridge transaction. It does not actually speed up the
            bridging process itself that you initiate with this transaction — you cannot speed up the 8-32 hour waiting
            time for a withdrawal.
          </p>
        </Collapse>

        <Collapse title="Where can I access support?">
          <p>Click the “Contact support” button in the bottom-left corner of the Linea Bridge app.</p>
          <p>Alternatively, head to the Linea Help Center and click the “Contact” button on the homepage.</p>
          <p>
            You can also get advice and support on our moderated{" "}
            <Link
              href="https://discord.gg/linea"
              rel="noopener noreferrer"
              passHref
              target="_blank"
              className="link link-secondary"
            >
              Discord
            </Link>
            .
          </p>
        </Collapse>
      </div>
    </div>
  );
}
