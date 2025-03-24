import localFont from "next/font/local";

const atypFont = localFont({
  display: "swap",
  src: [
    {
      path: "../../../public/fonts/AtypDisplay-Regular-subset.woff2",
      weight: "400",
      style: "normal",
    },
    {
      path: "../../../public/fonts/AtypDisplay-Medium-subset.woff2",
      weight: "500",
      style: "normal",
    },
    {
      path: "../../../public/fonts/AtypDisplay-Semibold-subset.woff2",
      weight: "600",
      style: "normal",
    },
    {
      path: "../../../public/fonts/AtypDisplay-Bold-subset.woff2",
      weight: "700",
      style: "normal",
    },
  ],
  variable: "--font-atyp",
});

export default atypFont;
