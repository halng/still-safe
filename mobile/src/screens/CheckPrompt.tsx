import { useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { ApiError, api } from "../api";
import { useAuth } from "../auth";
import { Body, Button, ErrorText, Title } from "../components/ui";
import { countdown, reasonDetail } from "../format";
import { Emergency } from "./PersonScreen";
import { colors } from "../theme";
import type { SafetyCheck } from "../types";

export function CheckPrompt({ check, onDone }: { check: SafetyCheck; onDone: () => void }) {
  const { token } = useAuth();
  const [now, setNow] = useState(Date.now());
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  async function confirm() {
    if (!token) return;
    setBusy(true);
    setError("");
    try {
      await api.confirmCheck(token, check.id);
      onDone();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not confirm.");
      onDone();
    } finally {
      setBusy(false);
    }
  }

  return (
    <View style={styles.cover}>
      <Title>Are you safe?</Title>
      <Body>
        {check.startedByName} asked. {reasonDetail[check.reason]}
      </Body>
      <Text style={styles.time}>{countdown(check.respondBy, now)}</Text>
      <Emergency reason={check.reason} self />
      <ErrorText>{error}</ErrorText>
      <Button title={busy ? "Sending" : "I'm safe"} onPress={confirm} disabled={busy} />
    </View>
  );
}

const styles = StyleSheet.create({
  cover: {
    position: "absolute",
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    backgroundColor: colors.bg,
    paddingHorizontal: 24,
    paddingTop: 72,
    zIndex: 20,
  },
  time: { marginTop: 20, color: colors.text, fontSize: 28, fontWeight: "500" },
});
