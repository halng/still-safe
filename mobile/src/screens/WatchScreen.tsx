import { useFocusEffect, useNavigation } from "@react-navigation/native";
import { NativeStackNavigationProp } from "@react-navigation/native-stack";
import { useCallback, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { ApiError, api } from "../api";
import { useAuth } from "../auth";
import { Body, Button, ErrorText, Field, Loading, Screen, Title } from "../components/ui";
import { accuracyLabel, ageLabel, reasonLabel } from "../format";
import { useLive } from "../live";
import { colors } from "../theme";
import type { RootStackParamList, Watched } from "../types";

export function WatchScreen() {
  const navigation = useNavigation<NativeStackNavigationProp<RootStackParamList>>();
  const { token } = useAuth();
  const [people, setPeople] = useState<Watched[] | null>(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    try {
      const result = await api.watching(token);
      setPeople(result.people ?? []);
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load people.");
      setPeople((current) => current ?? []);
    }
  }, [token]);

  useFocusEffect(
    useCallback(() => {
      load();
      const timer = setInterval(load, 15000);
      return () => clearInterval(timer);
    }, [load]),
  );

  useLive(
    useCallback(() => {
      load();
    }, [load]),
  );

  async function accept() {
    if (!token) return;
    setBusy(true);
    setError("");
    try {
      await api.acceptInvite(token, code.trim());
      setCode("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not accept the invite.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen scroll>
      <Title>People I watch</Title>
      <Body>Enter a code from someone who invited you.</Body>
      <Field label="Invite code" value={code} onChangeText={setCode} autoCapitalize="characters" />
      <Button title={busy ? "Accepting" : "Accept invite"} onPress={accept} disabled={busy || code.trim().length < 8} />
      <ErrorText>{error}</ErrorText>
      {people === null ? <Loading /> : null}
      {people && people.length === 0 ? <Text style={styles.empty}>No one has invited you yet.</Text> : null}
      {people?.map((person) => (
        <Pressable
          key={person.personId}
          accessibilityRole="button"
          accessibilityLabel={person.displayName}
          onPress={() => navigation.navigate("Person", { personId: person.personId })}
          style={styles.row}
        >
          <View>
            <Text style={styles.name}>{person.displayName}</Text>
            <Text style={styles.meta}>{statusLine(person)}</Text>
          </View>
          <Text style={styles.meta}>Open</Text>
        </Pressable>
      ))}
    </Screen>
  );
}

function statusLine(person: Watched): string {
  if (person.check?.status === "pending") return `Safety check · ${reasonLabel[person.check.reason]}`;
  if (person.check?.status === "unanswered") return "No confirmation";
  if (person.check?.status === "confirmed_safe") return "They confirmed they are safe";
  if (!person.sharingEnabled) return "Sharing is off";
  if (!person.lastLocation) return "No location yet";
  if (person.lastLocation.stale) return `Stale · ${ageLabel(person.lastLocation.recordedAt)}`;
  return `${ageLabel(person.lastLocation.recordedAt)} · ${accuracyLabel(person.lastLocation.accuracyM)}`;
}

const styles = StyleSheet.create({
  empty: { marginTop: 28, color: colors.secondary, fontSize: 16 },
  row: {
    marginTop: 20,
    paddingVertical: 14,
    borderTopWidth: 1,
    borderTopColor: colors.line,
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
  },
  name: { color: colors.text, fontSize: 18 },
  meta: { marginTop: 4, color: colors.secondary, fontSize: 14 },
});
