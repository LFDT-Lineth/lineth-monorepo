export const SUBMIT_INVOICE_ABI = [
  {
    inputs: [
      {
        internalType: "uint256",
        name: "_startTimestamp",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_endTimestamp",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_invoiceAmount",
        type: "uint256",
      },
    ],
    name: "submitInvoice",
    outputs: [],
    stateMutability: "payable",
    type: "function",
  },
] as const;

export const INVOICE_PROCESSED_EVENT_ABI = [
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        internalType: "address",
        name: "receiver",
        type: "address",
      },
      {
        indexed: true,
        internalType: "uint256",
        name: "startTimestamp",
        type: "uint256",
      },
      {
        indexed: true,
        internalType: "uint256",
        name: "endTimestamp",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "amountPaid",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "amountRequested",
        type: "uint256",
      },
    ],
    name: "InvoiceProcessed",
    type: "event",
  },
] as const;
