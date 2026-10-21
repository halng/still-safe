export const colors = {
  bg: "#FFFFFF",
  text: "#171717",
  secondary: "#737373",
  line: "#E5E5E5",
  danger: "#B42318",
  button: "#171717",
  buttonText: "#FFFFFF",
};

export const type = {
  title: { fontSize: 34, lineHeight: 40, fontWeight: "500" as const, color: colors.text, letterSpacing: -0.6 },
  body: { fontSize: 16, lineHeight: 22, color: colors.secondary },
  label: { fontSize: 13, lineHeight: 18, color: colors.secondary },
};
