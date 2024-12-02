import Link from "next/link";

type TransactionConfirmationModalProps = {
  handleClose: () => void;
};

const TransactionConfirmationModal: React.FC<TransactionConfirmationModalProps> = ({ handleClose }) => {
  return (
    <div className="flex flex-col items-center justify-center gap-8 px-8 py-4">
      <h2 className="text-xl">Transaction confirmed!</h2>
      <div className="flex items-center justify-center gap-4">
        <Link href="/" className="btn btn-primary rounded-full uppercase" onClick={handleClose}>
          Start a new transaction
        </Link>
        <Link
          className="border-1 btn btn-outline rounded-full border-primary uppercase hover:border-primary hover:bg-cardBg hover:text-card"
          href="/transactions"
          onClick={handleClose}
        >
          Track transactions
        </Link>
      </div>
    </div>
  );
};

export default TransactionConfirmationModal;
