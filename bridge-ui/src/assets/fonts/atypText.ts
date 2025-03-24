import localFont from "next/font/local";

const atypTextFont = localFont({
  display: "swap",
  src: [
    {
      path: "../../../public/fonts/AtypText-Regular-subset.woff2",
      weight: "400",
      style: "normal",
    },
    {
      path: "../../../public/fonts/AtypText-Medium-subset.woff2",
      weight: "500",
      style: "normal",
    },
    {
      path: "../../../public/fonts/AtypText-Semibold-subset.woff2",
      weight: "600",
      style: "normal",
    },
    {
      path: "../../../public/fonts/AtypText-Bold-subset.woff2",
      weight: "700",
      style: "normal",
    },
  ],
  variable: "--font-atyp-text",
});

export default atypTextFont;
