import { useState } from "react";
import Tooltip from "@/components/ui/tooltip";
import Copy from "@/assets/icons/copy.svg";
import Check from "@/assets/icons/check.svg";
import styles from "./copy-to-clipboard.module.scss";

type Props = {
  text: string;
  tooltipDefault?: string;
  children?: React.ReactNode;
};

export default function CopyToClipboard({ text, tooltipDefault = "Copy to clipboard", children }: Props) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 4000);
    } catch (err) {
      console.error("Failed to copy!", err);
    }
  };

  return (
    <Tooltip text={tooltipDefault}>
      <button className={styles.button} onClick={handleCopy}>
        {children || (copied ? <Check className={styles.icon} /> : <Copy className={styles.icon} />)}
      </button>
    </Tooltip>
  );
}
