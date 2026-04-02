import { ReactNode } from "react";

import type { Metadata } from "next";

import "./globals.css";

export const metadata: Metadata = {
  title: "Linea · Hardhat signer UI",
  description: "Local browser wallet signing for Linea contracts (Hardhat HARDHAT_SIGNER_UI).",
  icons: {
    icon: [{ url: "https://linea.build/favicon-32x32.png", sizes: "32x32", type: "image/png" }],
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body className="signer-ui-body">{children}</body>
    </html>
  );
}
