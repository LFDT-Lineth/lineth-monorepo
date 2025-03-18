"use client";
import { useState } from "react";
import { motion } from "framer-motion";

import styles from "./page.module.scss";
import Link from "next/link";
import ArrowLeftIcon from "@/assets/icons/arrow-left.svg";
import PlusIcon from "@/assets/icons/plus.svg";

const faqList = [
  {
    title: "Which address do the funds go to?",
    content: `<p>By default, your bridged funds are sent to the same address they were originally sent from.</p>`,
  },
  {
    title: "Can I send the funds to a different address?",
    content: `<p>Yes — find the 'To different address' button beneath the field that displays the amount of ETH or tokens you will receive on the target chain. You can then enter the address you want the funds to be bridged to. Only send funds to addresses on Ethereum Mainnet or Linea Mainnet. You may lose funds otherwise, and they will be irretrievable (see this <a rel="noopener noreferrer" target="_blank" href="https://support.metamask.io/managing-my-tokens/moving-your-tokens/funds-sent-on-wrong-network/#non-evm-tokens-and-networks">MetaMask Support article</a> for an explanation). This means you cannot bridge to Bitcoin or Solana, for example, using the Linea Bridge.</p>`,
  },
  {
    title: "I need to claim a transaction. What do I do?",
    content: `<div><p>If you chose manual claiming, you need to return to the Linea Bridge and make an additional transaction. It typically takes around 20 minutes (two Ethereum epochs) before you can claim a deposit; withdrawals from Linea can take much longer, and vary in duration.</p><p>To claim, head to the 'Transactions' tab and locate the bridge transaction you want to claim; it should be marked as 'Ready to claim'. Click on it, and then on the 'Claim' button to initiate the transaction.</p></div>`,
  },
  {
    title: "What fees will I pay?",
    content: `<div><p>Linea does not charge fees for using the bridge. However, you will need to pay gas fees for transactions. Gas fees vary according to network activity, so we cannot accurately predict how much you'll pay. The bulk of the cost will be for interacting with Ethereum Mainnet; Linea's gas fees are considerably cheaper.</p><p>If you use manual claiming, you pay gas fees for the bridge transaction and then gas fees for the claim transaction.</p><p>For automatic claiming, you pay gas fees for your initial bridge transaction and then a fee to cover the gas costs of the postman executing the claim on your behalf.</p></div>`,
  },
  {
    title: "Why can I only select Linea Mainnet and Ethereum Mainnet?",
    content: `<div><p>The Linea Bridge is designed to transact between layer 1 (Ethereum Mainnet) and layer 2 (Linea Mainnet), rather than for bridging to any other chains.</p><p>If you want to bridge to other EVM-compatible networks, consider using the <a rel="noopener noreferrer" target="_blank" href="https://portfolio.metamask.io/bridge">MetaMask Portfolio bridge</a>.</p></div>`,
  },
  {
    title: "Why can't I see the token I want to bridge?",
    content: `<p>The tokens available to select on the Linea Bridge are sourced from a curated list defined <a rel="noopener noreferrer" target="_blank" href="https://github.com/Consensys/linea-token-list/blob/main/json/linea-mainnet-token-shortlist.json">here</a> and maintained by the Linea team. This practice ensures users always bridge to the correct token—rather than variants, preventing <a rel="noopener noreferrer" target="_blank" href="https://support.linea.build/bridging/bridging-to-the-correct-erc-20-token-on-linea">liquidity fragmentation</a> and mitigates the risk of funds loss.</p>`,
  },
  {
    title: "Why can't I see my bridged tokens in my wallet?",
    content: `<div><p>First, check whether your funds are ready to claim. To see claimable funds, go to the “Transactions” tab of the Linea Bridge app.</p><p>If claiming isn't the issue, head to Lineascan and see if the transaction is pending, and in the queue:</p><ul class="list-disc pl-8"><li><a rel="noopener noreferrer" target="_blank" href="https://lineascan.build/txsDeposits">Here</a> for L1 -&gt; L2 transactions (deposits)</li><li><a rel="noopener noreferrer" target="_blank" href="https://lineascan.build/txsExit">Here</a> for L2 -&gt; L1 transactions (exit transactions)</li></ul><p>If the transaction is still pending, just wait for it to be confirmed, and your funds will be available to claim or will be in your wallet (depending on the claiming method you chose). If the transaction is missing, or it has been confirmed and you still don&amp;pos;t see your tokens, contact support by clicking the button in the bottom-left of the Linea Bridge app, or head to the <a rel="noopener noreferrer" target="_blank" href="https://support.linea.build/">Linea help center</a>.</p></div>`,
  },
  {
    title: "How long does the bridging take?",
    content: `<div><p>This depends on the direction of your bridge:</p><ul><li>Deposit (L1 -&gt; L2): Approximately 20 minutes.</li><li>Withdrawal (L2 -&gt; L1): Between 8 and 32 hours. The L2 transaction must be finalized on Ethereum Mainnet before you can claim your funds.</li></ul></div>`,
  },
  {
    title: "Can I speed up my transaction?",
    content: `<div><p>Yes, although it's not a method we recommend for beginners. <a rel="noopener noreferrer" target="_blank" href="https://support.linea.build/bridging/can-i-speed-up-bridge-transfers-on-the-linea-bridge">View the guide on the MetaMask Help Center</a>.</p><p>Note that this only speeds up your submission of the bridge transaction. It does not actually speed up the bridging process itself that you initiate with this transaction — you cannot speed up the 8-32 hour waiting time for a withdrawal.</p></div>`,
  },
  {
    title: "Where can I access support?",
    content: `<div><p>Click the “Contact support” button in the bottom-left corner of the Linea Bridge app.</p><p>Alternatively, head to the Linea Help Center and click the “Contact” button on the homepage.</p><p>You can also get advice and support on our moderated <a rel="noopener noreferrer" target="_blank" href="https://discord.gg/linea">Discord</a>.</p></div>`,
  },
];

export default function FaqPage() {
  const [openIndex, setOpenIndex] = useState<number | null>(null);

  const handleToggle = (index: number) => {
    setOpenIndex(openIndex === index ? null : index);
  };

  return (
    <>
      <div className={styles["content-wrapper"]}>
        <Link className={styles["home-link"]} href="/">
          <ArrowLeftIcon />
          <span>Bridge</span>
        </Link>
        <h1 className={styles["title"]}>FAQ</h1>

        <ul className={styles["list"]}>
          {faqList.map((faq, index) => (
            <FaqItem key={index} data={faq} isOpen={openIndex === index} onToggle={() => handleToggle(index)} />
          ))}
        </ul>
      </div>
    </>
  );
}

type FaqItemProps = {
  data: {
    title: string;
    content: string;
  };
  isOpen: boolean;
  onToggle: () => void;
};

function FaqItem({ data, isOpen, onToggle }: FaqItemProps) {
  const { title, content } = data;
  return (
    <li className={styles["item"]}>
      <motion.div
        className={styles["header"]}
        onClick={onToggle}
        role="button"
        tabIndex={0}
        aria-expanded={isOpen}
        onKeyDown={(e) => e.key === "Enter" && onToggle()}
        animate={{
          marginBottom: isOpen ? "1.6rem" : "0rem",
        }}
        transition={{
          marginBottom: {
            duration: 0.3,
            ease: "easeInOut",
          },
        }}
      >
        <h2 className={styles["item-title"]}>{title}</h2>
        <motion.div
          animate={{ rotate: isOpen ? 225 : 0 }}
          transition={{ duration: 0.3, ease: "easeInOut" }}
          className={styles["button"]}
        >
          <PlusIcon />
        </motion.div>
      </motion.div>

      <motion.div
        className={styles["content"]}
        initial={false}
        animate={{
          height: isOpen ? "auto" : 0,
          opacity: isOpen ? 1 : 0,
        }}
        transition={{
          duration: 0.3,
        }}
        style={{ overflow: "hidden" }}
      >
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: isOpen ? 1 : 0, y: isOpen ? 0 : -10 }}
          transition={{
            duration: 0.3,
            ease: "easeInOut",
            delay: isOpen ? 0.1 : 0,
          }}
          dangerouslySetInnerHTML={{ __html: content }}
        />
      </motion.div>
    </li>
  );
}
