import * as Location from "expo-location";
import { useCallback, useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { ApiError, api } from "../api";
import { useAuth } from "../auth";
import { Body, Button, ErrorText, Field, Loading, Screen, Title } from "../components/ui";
import { accuracyLabel, ageLabel } from "../format";
import { useLive } from "../live";
import { useBackgroundLimited } from "../location/bridge";
import { colors } from "../theme";
import type { Place, Supervisor } from "../types";

const suggestions = ["Home", "Work", "Clinic"];

export function CircleScreen() {
  const backgroundLimited = useBackgroundLimited();
  const { token, me, refresh, signOut } = useAuth();
  const [supervisors, setSupervisors] = useState<Supervisor[]>([]);
  const [places, setPlaces] = useState<Place[]>([]);
  const [code, setCode] = useState("");
  const [placeLabel, setPlaceLabel] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    try {
      const [people, saved] = await Promise.all([api.supervisors(token), api.places(token)]);
      setSupervisors(people.supervisors ?? []);
      setPlaces(saved.places ?? []);
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load your circle.");
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    load();
  }, [load]);

  useLive(
    useCallback(
      (event) => {
        if (me && event.personId === me.id && (event.type === "location" || event.type === "sharing")) {
          refresh().catch(() => undefined);
        }
      },
      [me, refresh],
    ),
  );

  async function run(action: () => Promise<void>) {
    if (!token) return;
    setBusy(true);
    setError("");
    try {
      await action();
      await refresh();
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong.");
    } finally {
      setBusy(false);
    }
  }

  async function toggleSharing() {
    if (!token || !me) return;
    if (me.sharingEnabled) {
      await run(() => api.setSharing(token, false).then(() => undefined));
      return;
    }
    const permission = await Location.requestForegroundPermissionsAsync();
    if (permission.status !== "granted") {
      setError("Location permission is off. Allow precise location to share it.");
      return;
    }
    await run(async () => {
      await api.setSharing(token, true);
      try {
        const position = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
        await api.postLocation(token, position);
      } catch {
        setError("Sharing is on. Circle is still waiting for a location fix.");
      }
    });
  }

  async function savePlace() {
    if (!token) return;
    const label = placeLabel.trim();
    if (!label) {
      setError("Name the place.");
      return;
    }
    const permission = await Location.requestForegroundPermissionsAsync();
    if (permission.status !== "granted") {
      setError("Location permission is off. Allow it to save where you are.");
      return;
    }
    await run(async () => {
      const position = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
      await api.createPlace(token, {
        label,
        latitude: position.coords.latitude,
        longitude: position.coords.longitude,
      });
      setPlaceLabel("");
    });
  }

  if (!me || loading) {
    return (
      <Screen>
        <Loading />
      </Screen>
    );
  }

  const last = me.lastLocation;
  const where = last
    ? `${last.stale ? "Stale · " : ""}${ageLabel(last.recordedAt)} · ${accuracyLabel(last.accuracyM)}`
    : "No location yet";

  return (
    <Screen scroll>
      <Title>My circle</Title>
      <Body>{me.sharingEnabled ? "Precise location is on." : "Precise location is off."}</Body>
      <Text style={styles.meta}>{where}</Text>
      {backgroundLimited && me.sharingEnabled ? (
        <Text style={styles.meta}>Updates continue while Circle is open.</Text>
      ) : null}
      <Button
        title={me.sharingEnabled ? "Stop sharing" : "Share location"}
        onPress={toggleSharing}
        disabled={busy}
      />

      <Text style={styles.section}>Supervisors · {me.activeSupervisors} of {me.supervisorLimit}</Text>
      <View style={styles.row}>
        <Button
          title="Fewer"
          tone="plain"
          disabled={busy || me.supervisorLimit <= 1}
          onPress={() => run(() => api.setLimit(token!, me.supervisorLimit - 1).then(() => undefined))}
        />
        <Button
          title="More"
          tone="plain"
          disabled={busy || me.supervisorLimit >= 5}
          onPress={() => run(() => api.setLimit(token!, me.supervisorLimit + 1).then(() => undefined))}
        />
      </View>
      {supervisors.length === 0 ? <Text style={styles.meta}>No supervisors yet.</Text> : null}
      {supervisors.map((person) => (
        <View key={person.linkId} style={styles.person}>
          <Text style={styles.name}>{person.displayName}</Text>
          <Button
            title="Revoke"
            tone="plain"
            disabled={busy}
            onPress={() => run(() => api.revoke(token!, person.linkId))}
          />
        </View>
      ))}
      {code ? (
        <View>
          <Text style={styles.code}>{code}</Text>
          <Text style={styles.meta}>Single use. Give this to someone you trust.</Text>
        </View>
      ) : (
        <Button title="Create an invite" disabled={busy} onPress={() => run(async () => setCode((await api.createInvite(token!)).code))} />
      )}

      <Text style={styles.section}>Common places</Text>
      <View style={styles.row}>
        {suggestions.map((label) => (
          <Button key={label} title={label} tone="plain" onPress={() => setPlaceLabel(label)} />
        ))}
      </View>
      <Field label="Place" value={placeLabel} onChangeText={setPlaceLabel} autoCapitalize="words" />
      <Button title="Save where I am" onPress={savePlace} disabled={busy} />
      {places.length === 0 ? <Text style={styles.meta}>No saved places.</Text> : null}
      {places.map((place) => (
        <View key={place.id} style={styles.person}>
          <Text style={styles.name}>{place.label}</Text>
          <Button title="Remove" tone="plain" disabled={busy} onPress={() => run(() => api.deletePlace(token!, place.id))} />
        </View>
      ))}

      <ErrorText>{error}</ErrorText>
      <Button title="Sign out" tone="plain" onPress={signOut} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  meta: { marginTop: 8, color: colors.secondary, fontSize: 15, lineHeight: 21 },
  section: { marginTop: 36, color: colors.text, fontSize: 20, fontWeight: "500" },
  row: { flexDirection: "row", flexWrap: "wrap" },
  person: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", borderBottomWidth: 1, borderBottomColor: colors.line },
  name: { color: colors.text, fontSize: 16, flexShrink: 1 },
  code: { marginTop: 28, color: colors.text, fontSize: 28, letterSpacing: 4 },
});
