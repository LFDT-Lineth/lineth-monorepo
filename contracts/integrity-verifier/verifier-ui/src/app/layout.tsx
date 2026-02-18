import type { Metadata } from "next";
import "@/scss/app.scss";

export const metadata: Metadata = {
  title: "Contract Integrity Verifier",
  description: "Verify deployed smart contracts against local artifacts",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <main>{children}</main>
      </body>
    </html>
  );
}
