import { NativeStackScreenProps } from "@react-navigation/native-stack";
import { useCallback, useEffect, useState } from "react";
import { Linking, StyleSheet, Text, View } from "react-native";
import { ApiError, api } from "../api";
import { useAuth } from "../auth";
import { mapHtml } from "../components/mapHtml";
import { MapPanel } from "../components/MapPanel";
import { Body, Button, ErrorText, Loading, Screen } from "../components/ui";
import { accuracyLabel, ageLabel, clock, countdown, reasonLabel } from "../format";
import { useLive } from "../live";
import { colors } from "../theme";
import type { PersonDetail, Reason, RootStackParamList } from "../types";

const reasons: Reason[] = ["not_responding", "unwell", "self_harm", "missing"];

export function PersonScreen({ route, navigation }: NativeStackScreenProps<RootStackParamList, "Person">) {
  const { token } = useAuth();
  const [person, setPerson] = useState<PersonDetail | null>(null);
  const [error, setError] = useState("");
  const [choosing, setChoosing] = useState(false);
  const [now, setNow] = useState(Date.now());

  const load = useCallback(async () => {
    if (!token) return;
    try {
      setPerson(await api.person(token, route.params.personId));
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load this person.");
    }
  }, [token, route.params.personId]);

  useEffect(() => {
    load();
    const timer = setInterval(() => {
      setNow(Date.now());
      load();
    }, 10000);
    return () => clearInterval(timer);
  }, [load]);

  useLive(
    useCallback(
      (event) => {
        if (event.personId === route.params.personId) load();
      },
      [load, route.params.personId],
    ),
  );

  async function start(reason: Reason) {
    if (!token) return;
    setError("");
    try {
      await api.startCheck(token, route.params.personId, reason);
      setChoosing(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start the check.");
    }
  }

  if (!person && !error) {
    return (
      <Screen>
        <Loading />
      </Screen>
    );
  }
  if (!person) {
    return (
      <Screen>
        <ErrorText>{error}</ErrorText>
        <Button title="Back" tone="plain" onPress={() => navigation.goBack()} />
      </Screen>
    );
  }

  const hasMap = Boolean(person.lastLocation || person.trail.length || person.places.length);
  const check = person.check;
  const unanswered = check?.status === "unanswered";

  return (
    <View style={styles.fill}>
      <View style={styles.map}>
        {hasMap ? (
          <MapPanel html={mapHtml({ location: person.lastLocation, trail: person.trail, places: person.places })} />
        ) : (
          <View style={styles.emptyMap}>
            <Text style={styles.meta}>No location yet.</Text>
          </View>
        )}
      </View>
      <Screen scroll style={styles.sheet}>
        <Button title="Back" tone="plain" onPress={() => navigation.goBack()} />
        <Text style={[styles.name, unanswered && styles.danger]}>{unanswered ? "No confirmation yet." : person.displayName}</Text>
        {unanswered ? <Text style={styles.nameQuiet}>{person.displayName}</Text> : null}
        <Body>{locationLine(person)}</Body>
        {person.places.length > 0 ? (
          <Text style={styles.meta}>Places · {person.places.map((place) => place.label).join(", ")}</Text>
        ) : (
          <Text style={styles.meta}>No saved places.</Text>
        )}
        {check?.status === "pending" ? (
          <Text style={styles.meta}>
            Waiting for them to confirm they are safe · {reasonLabel[check.reason]} · {countdown(check.respondBy, now)}
          </Text>
        ) : null}
        {check?.status === "confirmed_safe" ? (
          <Text style={styles.meta}>They confirmed they are safe at {clock(check.resolvedAt)}.</Text>
        ) : null}
        {unanswered ? <Emergency reason={check?.reason} /> : null}
        {check?.status === "pending" ? null : choosing ? (
          <View>
            {reasons.map((reason) => (
              <Button key={reason} title={reasonLabel[reason]} tone="plain" onPress={() => start(reason)} />
            ))}
          </View>
        ) : (
          <Button title="Start a safety check" onPress={() => setChoosing(true)} />
        )}
        <ErrorText>{error}</ErrorText>
      </Screen>
    </View>
  );
}

function locationLine(person: PersonDetail): string {
  if (!person.sharingEnabled && person.lastLocation) {
    return `Sharing is off. Last point ${ageLabel(person.lastLocation.recordedAt)}, ${accuracyLabel(person.lastLocation.accuracyM)}.`;
  }
  if (!person.sharingEnabled) return "Sharing is off.";
  if (!person.lastLocation) return "No location yet.";
  const prefix = person.lastLocation.stale ? "Last location is stale. " : "";
  return `${prefix}${ageLabel(person.lastLocation.recordedAt)} · ${accuracyLabel(person.lastLocation.accuracyM)}`;
}

export function Emergency({ reason, self = false }: { reason?: Reason; self?: boolean }) {
  return (
    <View>
      <Text style={styles.dangerBody}>
        {self
          ? "If you are in danger, contact local emergency services."
          : "If they are in danger, contact local emergency services."}
      </Text>
      <Button title="Call emergency services" tone="danger" onPress={() => Linking.openURL("tel:911")} />
      {reason === "self_harm" ? (
        <>
          <Text style={styles.meta}>988 is the US Suicide & Crisis Lifeline.</Text>
          <Button title="Call 988" tone="danger" onPress={() => Linking.openURL("tel:988")} />
        </>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1, backgroundColor: colors.bg },
  map: { flex: 1, minHeight: 220, backgroundColor: colors.bg },
  emptyMap: { flex: 1, alignItems: "center", justifyContent: "center" },
  sheet: { flexGrow: 0, paddingTop: 8 },
  name: { marginTop: 8, color: colors.text, fontSize: 28, fontWeight: "500" },
  nameQuiet: { marginTop: 4, color: colors.text, fontSize: 16 },
  meta: { marginTop: 8, color: colors.secondary, fontSize: 15, lineHeight: 21 },
  danger: { color: colors.danger },
  dangerBody: { marginTop: 12, color: colors.danger, fontSize: 16, lineHeight: 22 },
});
