type TransactionDetailRowProps = {
  label: string;
  value: React.ReactNode | string;
};

const TransactionDetailRow: React.FC<TransactionDetailRowProps> = ({ label, value }) => (
  <div className="flex items-center">
    <label className="w-44 text-neutral-600">{label}</label>
    <span>{value}</span>
  </div>
);

export default TransactionDetailRow;
