import type { Config } from "tailwindcss";
// @ts-ignore - plugin has no types
import forms from "@tailwindcss/forms";

const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        navy: "#0f172a",
        cyan: "#22d3ee",
        sky: "#0ea5e9",
      },
    },
  },
  plugins: [forms],
};

export default config;
