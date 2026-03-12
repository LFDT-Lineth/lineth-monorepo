import clsx from "clsx";
import Link from "next/link";

import { useChainStore } from "@/stores/chainStore";

import styles from "./page-footer.module.scss";

type PageFooterProps = {
  showYieldBoost?: boolean;
};

export default function PageFooter({ showYieldBoost = false }: PageFooterProps) {
  const fromChain = useChainStore.useFromChain();
  const toChain = useChainStore.useToChain();
  const isMainnet = !fromChain.testnet && !toChain.testnet;

  return (
    <>
      {showYieldBoost && isMainnet && (
        <div className={clsx(styles["page-footer"])}>
          ETH bridged to Linea are being staked with Yield Boost.{" "}
          <Link
            data-testid="yield-boost-link"
            href="https://docs.linea.build/network/overview/yield-boost"
            target="_blank"
            rel="noopener noreferrer"
          >
            Learn more.
          </Link>
        </div>
      )}
      <div className={clsx(styles["page-footer"])}>
        Need help?{" "}
        <Link data-testid="faq-page-link" href="/faq">
          Check our FAQ
        </Link>
      </div>
    </>
  );
}
