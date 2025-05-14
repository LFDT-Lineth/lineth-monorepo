"use client";
import { useState } from "react";
import { motion } from "motion/react";

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
    content: `<p>Yes — find the wallet icon in the bottom-right of the widget. Click it to display a field where you can enter a new recipient address.</p>`,
  },
  {
    title: "Why do I need to claim my funds?",
    content: `<p>Manual claiming is required for all L2 → L1 transfers, and in some other circumstances detailed in our <a rel="noopener noreferrer" target="_blank" href="https://docs.linea.build/get-started/how-to/bridge">documentation</a>.</p>`,
  },
  {
    title: "I need to claim a transaction. What do I do?",
    content: `<div><p>If your transfer requires manual claiming, you need to return to the native bridge and make an additional transaction. It typically takes around 20 minutes before you can claim a deposit; withdrawals from Linea can take longer.</p><p>To claim, click the receipt icon in the top-right of the widget. If your transaction is ready, it will be marked as “Ready to claim”. Click the transaction, and then “Claim” to prompt the claiming transaction in your wallet. Once it’s complete, the funds will be in your wallet.</p></div>`,
  },
  {
    title: "What fees will I pay?",
    content: `<div><p>Linea does not charge fees for using the native bridge. However, you will still need to pay gas fees on Ethereum when bridging from Ethereum to Linea.</p><p>In the default automatic claiming flow from Ethereum to Linea, we cover the gas fees on the Linea side, which means you only have to pay gas on Ethereum and nothing more.</p><p>If you choose manual claiming, you will need to pay gas fees on both Ethereum and Linea.</p></div>`,
  },
  {
    title: "Why can I only select Linea Mainnet and Ethereum Mainnet?",
    content: `<div><p>The native bridge is designed for transacting between layer 1 (Ethereum Mainnet) and layer 2 (Linea Mainnet) only.</p><p>If you want to bridge to other chains, switch to the “Bridge Aggregator” tab.</p></div>`,
  },
  {
    title: "Why can't I see the token I want to bridge?",
    content: `<p>Available tokens are sourced from a curated list defined <a href="https://github.com/Consensys/linea-token-list/blob/main/json/linea-mainnet-token-shortlist.json" target="_blank" rel="noopener noreferrer">here</a> and maintained by the Linea team. This ensures users always bridge to the correct token rather than variants, preventing <a href="https://support.linea.build/bridging/bridging-to-the-correct-erc-20-token-on-linea" target="_blank" rel="noopener noreferrer">liquidity fragmentation</a>, and mitigates the risk of funds loss.</p>`,
  },
  {
    title: "Why can't I see my bridged tokens in my wallet?",
    content: `<div><p>First, check whether your funds are ready to claim. To see claimable funds, go to the transaction history page, accessible via the receipt icon in the top-right of the widget.</p><p>If claiming isn't the issue, head to LineaScan and see if the transaction is pending, and in the queue:</p><ul class="list-disc pl-8"><li><a rel="noopener noreferrer" target="_blank" href="https://lineascan.build/txsDeposits">Here</a> for L1 -&gt; L2 transactions (deposits)</li><li><a rel="noopener noreferrer" target="_blank" href="https://lineascan.build/txsExit">Here</a> for L2 -&gt; L1 transactions (exit transactions)</li></ul><p>If the transaction is still pending, wait for it to be confirmed, and your funds will be available to claim or will be in your wallet (depending on the claiming method you chose). If the transaction is missing, or it has been confirmed and you still don’t see your tokens, contact support via the “Contact” button on the <a rel="noopener noreferrer" target="_blank" href="https://support.linea.build/">Linea help center homepage</a>.</p></div>`,
  },
  {
    title: "How long does bridging take?",
    content: `<div><p>This depends on the direction of your bridge:</p><ul><li>Deposit (L1 -&gt; L2): Approximately 20 minutes.</li><li>Withdrawal (L2 -&gt; L1): Between 2 and 16 hours. The L2 transaction must be finalized on Ethereum Mainnet before you can claim your funds.</li></ul></div>`,
  },
  {
    title: "Can I speed up my transaction?",
    content: `<div><p>Yes, although it's not a method we recommend for beginners. <a rel="noopener noreferrer" target="_blank" href="https://support.linea.build/bridging/can-i-speed-up-bridge-transfers-on-the-linea-bridge">View the guide on the Linea help center</a>.</p><p>Note that this only speeds up your submission of the bridge transaction. It does not actually speed up the bridging process itself that you initiate with this transaction — you cannot speed up the 8-32 hour waiting time for a withdrawal.</p></div>`,
  },
  {
    title: "Where can I access support?",
    content: `<div><p>Head to the <a href="https://support.linea.build/" target="_blank" rel="noopener noreferrer">Linea help center</a> and click the “Contact” button on the homepage, or the chat icon in the bottom right.</p><p>You can also get advice and support on our moderated <a href="https://discord.gg/linea" target="_blank" rel="noopener noreferrer">Discord</a>.</p></div>`,
  },
  {
    title: "Can I bridge testnet funds?",
    content: `<div><p>Yes, you can use Sepolia and Linea Sepolia with the native bridge. These are the only testnets supported.</p><p>Click the gear icon to access the settings, then flick the “Show Test Networks” toggle.</p></div>`,
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
            <FaqItem
              key={`faq-item-${index}`}
              data={faq}
              isOpen={openIndex === index}
              onToggle={() => handleToggle(index)}
            />
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
