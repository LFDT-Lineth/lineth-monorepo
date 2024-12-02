import type { Config } from "tailwindcss";
import daisyui from "daisyui";
import tailwindScrollbar from "tailwind-scrollbar";
import daisyuiThemes from "daisyui/src/theming/themes";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          light: "#C4F3FF",
          DEFAULT: "#61DFFF",
        },
        secondary: {
          light: "#EBE2FD",
          DEFAULT: "#6119EF",
        },
        orange: {
          light: "#FFF1E9",
          DEFAULT: "#FF8D4C",
        },
        card: "#505050",
        cardBg: "#FFFFFF",
        backgroundColor: "var(--background-color)",
        icon: "#C0C0C0",
        yellow: "var(--yellow)",
        success: "#C1FF14",
      },
      fontFamily: {
        atyp: ["var(--font-atyp)"],
        atypText: ["var(--font-atyp-text)"],
      },
    },
  },

  daisyui: {
    themes: [
      {
        light: {
          ...daisyuiThemes.light,
          primary: "#61DFFF",
          secondary: "#6119EF",
          warning: "#FF8D4C",
          info: "#61DFFF",
          success: "#C1FF14",
        },
      },
    ],
  },
  plugins: [daisyui, tailwindScrollbar],
};

export default config;
