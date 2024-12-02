import { formatHex } from "@/utils/format";
import Link from "next/link";
import { BsDashLg } from "react-icons/bs";

type BlockExplorerLinkProps = {
  transactionHash?: string;
  blockExplorer?: string;
};

const BlockExplorerLink: React.FC<BlockExplorerLinkProps> = ({ transactionHash, blockExplorer }) => {
  if (!transactionHash || !blockExplorer) {
    return <BsDashLg />;
  }
  return (
    <Link
      href={`${blockExplorer}/tx/${transactionHash}`}
      passHref
      target="_blank"
      rel="noopener noreferrer"
      className="link text-secondary"
    >
      {formatHex(transactionHash)}
    </Link>
  );
};

export default BlockExplorerLink;
