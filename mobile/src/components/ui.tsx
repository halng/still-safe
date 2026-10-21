import { ReactNode } from "react";
import {
  ActivityIndicator,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
  ViewStyle,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { colors, type } from "../theme";

export function Screen({
  children,
  scroll = false,
  style,
}: {
  children: ReactNode;
  scroll?: boolean;
  style?: ViewStyle;
}) {
  const inner = <View style={[styles.inner, style]}>{children}</View>;
  return (
    <SafeAreaView style={styles.safe}>
      {scroll ? (
        <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">
          {inner}
        </ScrollView>
      ) : (
        inner
      )}
    </SafeAreaView>
  );
}

export function Title({ children }: { children: string }) {
  return <Text style={type.title}>{children}</Text>;
}

export function Body({ children }: { children: ReactNode }) {
  return <Text style={[type.body, styles.body]}>{children}</Text>;
}

export function Field({
  label,
  value,
  onChangeText,
  secureTextEntry,
  autoCapitalize = "sentences",
  placeholder,
  keyboardType,
}: {
  label: string;
  value: string;
  onChangeText: (value: string) => void;
  secureTextEntry?: boolean;
  autoCapitalize?: "none" | "sentences" | "words" | "characters";
  placeholder?: string;
  keyboardType?: "default" | "number-pad";
}) {
  return (
    <View>
      <Text style={[type.label, styles.fieldLabel]}>{label}</Text>
      <TextInput
        value={value}
        onChangeText={onChangeText}
        secureTextEntry={secureTextEntry}
        autoCapitalize={autoCapitalize}
        autoCorrect={false}
        placeholder={placeholder}
        placeholderTextColor="#A3A3A3"
        keyboardType={keyboardType}
        style={styles.input}
      />
    </View>
  );
}

export function Button({
  title,
  onPress,
  disabled,
  tone = "dark",
}: {
  title: string;
  onPress: () => void;
  disabled?: boolean;
  tone?: "dark" | "danger" | "plain";
}) {
  const danger = tone === "danger";
  const plain = tone === "plain";
  return (
    <Pressable
      accessibilityRole="button"
      disabled={disabled}
      onPress={onPress}
      style={[styles.button, danger && styles.dangerButton, plain && styles.plainButton, disabled && styles.disabled]}
    >
      <Text style={[styles.buttonText, danger && styles.dangerText, plain && styles.plainText]}>{title}</Text>
    </Pressable>
  );
}

export function ErrorText({ children }: { children: string }) {
  if (!children) return null;
  return <Text style={styles.error}>{children}</Text>;
}

export function Loading({ label = "Loading" }: { label?: string }) {
  return (
    <View style={styles.loading}>
      <ActivityIndicator color={colors.text} />
      <Text style={[type.body, styles.loadingLabel]}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },
  scroll: { flexGrow: 1 },
  inner: { flexGrow: 1, paddingHorizontal: 24, paddingTop: 28, paddingBottom: 32 },
  body: { marginTop: 12 },
  fieldLabel: { marginTop: 22 },
  input: {
    borderBottomWidth: 1,
    borderBottomColor: colors.line,
    color: colors.text,
    fontSize: 16,
    paddingVertical: 12,
  },
  button: {
    marginTop: 16,
    minHeight: 52,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: colors.button,
    paddingHorizontal: 16,
  },
  dangerButton: { backgroundColor: colors.bg, borderWidth: 1, borderColor: colors.danger },
  plainButton: { backgroundColor: "transparent", minHeight: 44 },
  disabled: { opacity: 0.4 },
  buttonText: { color: colors.buttonText, fontSize: 16, fontWeight: "500" },
  dangerText: { color: colors.danger },
  plainText: { color: colors.text },
  error: { color: colors.danger, fontSize: 15, lineHeight: 21, marginTop: 16 },
  loading: { flex: 1, alignItems: "center", justifyContent: "center", gap: 12 },
  loadingLabel: { color: colors.secondary },
});
