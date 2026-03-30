"use client";
import { useState, type ReactNode } from "react";

import { motion } from "motion/react";

import PlusIcon from "@/assets/icons/plus.svg";

import styles from "./page.module.scss";

function ExternalLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  );
}

const faqList = [
  {
    title: "Which address do the funds go to?",
    content: (
      <p>
        By default, funds that you bridge using the Linea native bridge are sent to the same address they were
        originally sent from.
      </p>
    ),
  },
  {
    title: "Can I send the funds to a different address?",
    content: (
      <p>
        Yes, find the wallet icon in the bottom-right of the Linea native bridge widget. Click it to display a field
        where you can enter a new recipient address.
      </p>
    ),
  },
  {
    title: "I need to claim a transaction on the Linea native bridge. What do I do?",
    content: (
      <>
        <p>
          If your transfer requires manual claiming, you need to return to the Linea native bridge and make an
          additional transaction. It typically takes around 20 minutes (two Ethereum epochs) before you can claim a
          deposit; withdrawals from Linea can take much longer, and vary in duration.
        </p>
        <p>
          To claim, click the receipt icon in the top-right of the widget. If your transaction is ready, it will be
          marked as &ldquo;Ready to claim&rdquo;. Click the transaction, and then &ldquo;Claim&rdquo; to prompt the
          claiming transaction in your wallet. Once it&apos;s complete, the funds will be in your wallet.
        </p>
      </>
    ),
  },
  {
    title: "What fees will I pay on the Linea native bridge?",
    content: (
      <>
        <p>
          Linea does not charge fees for using the native bridge. However, you will need to pay gas fees for
          transactions. Gas fees vary according to network activity, so we cannot accurately predict how much
          you&apos;ll pay. The bulk of the cost will be for interacting with Ethereum Mainnet; gas fees on Linea are
          considerably cheaper.
        </p>
        <p>
          If you use manual claiming, you pay gas fees for the bridge transaction and then gas fees for the claim
          transaction.
        </p>
        <p>
          For automatic claiming, you pay gas fees for your initial bridge transaction and, in some cases, a fee to
          cover the gas costs of the postman executing the claim on your behalf. The postman fee is waived when
          depositing to Linea Mainnet.
        </p>
      </>
    ),
  },
  {
    title: "Why can I only select Linea Mainnet and Ethereum Mainnet?",
    content: (
      <>
        <p>
          The Linea native bridge is designed for transacting between layer 1 (Ethereum Mainnet) and layer 2 (Linea
          Mainnet) only.
        </p>
        <p>
          If you want to bridge to other chains, switch to the &ldquo;All Bridges&rdquo; tab, or head to the{" "}
          <ExternalLink href="https://linea.build/apps">dapp directory</ExternalLink> to find a third-party bridge.
        </p>
      </>
    ),
  },
  {
    title: "Why can't I see the token I want to bridge?",
    content: (
      <p>
        Available tokens are sourced from a curated list maintained{" "}
        <ExternalLink href="https://github.com/Consensys/linea-token-list/blob/main/json/linea-mainnet-token-shortlist.json">
          here
        </ExternalLink>
        . This ensures users always bridge to the correct token rather than variants, preventing{" "}
        <ExternalLink href="https://support.linea.build/bridging/bridging-to-the-correct-erc-20-token-on-linea">
          liquidity fragmentation
        </ExternalLink>
        , and mitigates the risk of funds loss.
      </p>
    ),
  },
  {
    title: "Why can't I see my tokens in my wallet after using the Linea native bridge?",
    content: (
      <>
        <p>
          First, check whether your funds are ready to claim. To see claimable funds, go to the transaction history
          page, accessible via the receipt icon in the top-right of the Linea native bridge widget.
        </p>
        <p>
          If claiming isn&apos;t the issue, head to LineaScan and see if the transaction is pending, and in the queue:
        </p>
        <ul>
          <li>
            <ExternalLink href="https://lineascan.build/txsDeposits">Here</ExternalLink> for L1 to L2 transactions
            (deposits)
          </li>
          <li>
            <ExternalLink href="https://lineascan.build/txsExit">Here</ExternalLink> for L2 to L1 transactions (exit
            transactions)
          </li>
        </ul>
        <p>
          If the transaction is still pending, wait for it to be confirmed, and your funds will be available to claim or
          will be in your wallet (depending on the claiming method you chose). If the transaction is missing, or it has
          been confirmed and you still don&apos;t see your tokens, contact support via the &ldquo;Contact&rdquo; button
          on the <ExternalLink href="https://support.linea.build/">Linea help center homepage</ExternalLink>.
        </p>
      </>
    ),
  },
  {
    title: "How long does the Linea native bridge take?",
    content: (
      <>
        <p>This depends on the asset and direction of your bridge transfer.</p>
        <p>For ETH or ERC-20 tokens:</p>
        <ul>
          <li>Deposit (L1 to L2): ~20 minutes.</li>
          <li>Withdrawal (L2 to L1): 2-12 hours.</li>
        </ul>
        <p>
          For USDC, deposits and withdrawals leverage the Circle Cross-Chain Transfer Protocol (CCTP) for much faster
          transfers.
        </p>
      </>
    ),
  },
  {
    title: "Can I speed up my transactions on the Linea native bridge?",
    content: (
      <p>
        Yes, although it&apos;s not a method we recommend for beginners.{" "}
        <ExternalLink href="https://support.linea.build/bridging/can-i-speed-up-bridge-transfers-on-the-linea-bridge">
          View the guide on the Linea help center
        </ExternalLink>
        . Note that this only speeds up your submission of the bridge transaction. It does not actually speed up the
        bridging process itself that you initiate with this transaction, you cannot speed up the much longer waiting
        time for a withdrawal on the Linea native bridge.
      </p>
    ),
  },
  {
    title: "Where can I access support for the Linea native bridge?",
    content: (
      <p>
        Head to the <ExternalLink href="https://support.linea.build/">Linea help center</ExternalLink> and click the
        &ldquo;Contact&rdquo; button on the homepage, or the chat icon in the bottom right for advice and support.
      </p>
    ),
  },
  {
    title: "Can I bridge testnet funds like Linea Sepolia and Sepolia?",
    content: (
      <>
        <p>Yes, you can use Sepolia and Linea Sepolia with the native bridge. These are the only testnets supported.</p>
        <p>Click the gear icon to access the settings, then flick the &ldquo;Show Test Networks&rdquo; toggle.</p>
      </>
    ),
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
    content: ReactNode;
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
        >
          {content}
        </motion.div>
      </motion.div>
    </li>
  );
}
