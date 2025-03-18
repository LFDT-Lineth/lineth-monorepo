import { Metadata } from "next";
import Script from "next/script";
import clsx from "clsx";
import usabillaBeScript from "@/scripts/usabilla";
import { gtmScript, gtmNoScript } from "@/scripts/gtm";
import { Providers } from "@/components/layouts/Providers";
import { Layout } from "@/components/layouts/Layout";
import atypFont from "@/assets/fonts/atyp";
import atypTextFont from "@/assets/fonts/atypText";
import "./globals.css";
import "../scss/app.scss";

const metadata: Metadata = {
  title: "Linea Bridge",
  description: `Linea Bridge is a bridge solution, providing secure and efficient cross-chain transactions between Layer 1 and Linea networks.
  Discover the future of blockchain interaction with Linea Bridge.`,
  icons: {
    icon: "./favicon.png",
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" data-theme="v2">
      <title>{metadata.title?.toString()}</title>
      <meta name="description" content={metadata.description?.toString()} key="desc" />

      <body className={clsx(atypFont.variable, atypTextFont.variable, atypFont.className, atypTextFont.className)}>
        <noscript dangerouslySetInnerHTML={{ __html: gtmNoScript }} />

        <Providers>
          <Layout>{children}</Layout>
        </Providers>
        <svg style={{ display: "none" }} viewBox="0 0 9 9" xmlns="http://www.w3.org/2000/svg">
          <defs>
            <symbol id="icon-new-tab" viewBox="0 0 9 9" fill="none">
              <path
                fillRule="evenodd"
                clipRule="evenodd"
                d="M6.95232 0.500055L8.4888 2.03654V6.57541L7.31527 6.57541V2.52263L7.3056 2.51296L1.31858 8.49998L0.48877 7.67016L6.47579 1.68315L6.46622 1.67358L2.37962 1.67356L2.37963 0.500031L6.95232 0.500055Z"
                fill="currentColor"
              />
            </symbol>
          </defs>
        </svg>
      </body>

      <Script id="usabilla" dangerouslySetInnerHTML={{ __html: usabillaBeScript }} />
      <Script id="gtm" dangerouslySetInnerHTML={{ __html: gtmScript }} />
    </html>
  );
}
