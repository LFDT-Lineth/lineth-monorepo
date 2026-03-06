import clsx from "clsx";
import { Metadata } from "next";
import { headers } from "next/headers";
import Script from "next/script";

import atypFont from "@/assets/fonts/atyp";
import atypTextFont from "@/assets/fonts/atypText";
import { Layout } from "@/components/layouts/Layout";
import { Providers } from "@/components/layouts/Providers";
import FirstVisitModal from "@/components/modal/first-time-visit";
import TosModal from "@/components/modal/tos-modal";
import { ModalBase } from "@/components/modal-base";
import { getNavData } from "@/lib/nav-data";
import { gtmScript } from "@/scripts/gtm";

import "../scss/app.scss";

export const metadata: Metadata = {
  title: "Linea Bridge",
  description: `Linea Bridge is a bridge solution, providing secure and efficient cross-chain transactions between Layer 1 and Linea networks.
  Discover the future of blockchain interaction with Linea Bridge.`,
};

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const headersList = await headers();
  const nonce = headersList.get("x-nonce") || "";
  const navData = await getNavData();

  return (
    <html lang="en" className={clsx(atypFont.variable, atypTextFont.variable)}>
      <meta name="theme-color" content="#ffffff" />
      <title>{metadata.title?.toString()}</title>
      <meta name="description" content={metadata.description?.toString()} key="desc" />

      <body>
        <noscript>
          <iframe
            src="https://www.googletagmanager.com/ns.html?id=GTM-PPCSK62D"
            height="0"
            width="0"
            style={{ display: "none", visibility: "hidden" }}
            title="Google Tag Manager"
          />
        </noscript>

        <Providers>
          <Layout navData={navData}>
            {children}
            <ModalBase />
          </Layout>
        </Providers>
        <FirstVisitModal />
        {/* TODO: Remove TosModal after March 28, 2026 */}
        <TosModal />
      </body>

      <Script id="gtm" dangerouslySetInnerHTML={{ __html: gtmScript }} strategy="lazyOnload" nonce={nonce} />
    </html>
  );
}
