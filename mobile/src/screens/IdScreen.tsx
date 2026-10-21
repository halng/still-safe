import * as ImagePicker from "expo-image-picker";
import { useState } from "react";
import { Platform, StyleSheet, Text, View } from "react-native";
import { ApiError, api } from "../api";
import { useAuth } from "../auth";
import { Body, Button, ErrorText, Field, Screen, Title } from "../components/ui";
import { colors } from "../theme";

const documents = [
  { id: "passport", label: "Passport" },
  { id: "national_id", label: "National ID" },
  { id: "drivers_license", label: "Driver's license" },
] as const;

type DocumentType = (typeof documents)[number]["id"];

export function IdScreen() {
  const { token, signIn, signOut } = useAuth();
  const [documentType, setDocumentType] = useState<DocumentType>("passport");
  const [legalName, setLegalName] = useState("");
  const [dateOfBirth, setDateOfBirth] = useState("");
  const [documentNumber, setDocumentNumber] = useState("");
  const [photo, setPhoto] = useState<ImagePicker.ImagePickerAsset | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function pick(camera: boolean) {
    setError("");
    const permission = camera
      ? await ImagePicker.requestCameraPermissionsAsync()
      : await ImagePicker.requestMediaLibraryPermissionsAsync();
    if (!permission.granted) {
      setError(camera ? "Camera permission is off." : "Photo permission is off.");
      return;
    }
    const result = camera
      ? await ImagePicker.launchCameraAsync({ mediaTypes: ["images"], quality: 0.7 })
      : await ImagePicker.launchImageLibraryAsync({ mediaTypes: ["images"], quality: 0.7 });
    if (!result.canceled) setPhoto(result.assets[0]);
  }

  async function submit() {
    if (!token || !photo) {
      setError("Add a photo of your ID.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const form = new FormData();
      form.append("legalName", legalName.trim());
      form.append("dateOfBirth", dateOfBirth.trim());
      form.append("documentType", documentType);
      form.append("documentNumber", documentNumber.trim());
      if (Platform.OS === "web") {
        const blob = await (await fetch(photo.uri)).blob();
        form.append("photo", blob, "id.jpg");
      } else {
        form.append("photo", {
          uri: photo.uri,
          name: "id.jpg",
          type: photo.mimeType || "image/jpeg",
        } as unknown as Blob);
      }
      await api.submitId(token, form);
      await signIn(token);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not save the ID.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen scroll>
      <Title>Your ID</Title>
      <Body>
        Age is taken from the date of birth on this ID. The photo is stored privately and is not shown to your
        supervisors.
      </Body>
      <Text style={styles.choiceLabel}>Document</Text>
      {documents.map((item) => (
        <Button
          key={item.id}
          title={documentType === item.id ? `${item.label} · selected` : item.label}
          tone={documentType === item.id ? "dark" : "plain"}
          onPress={() => setDocumentType(item.id)}
        />
      ))}
      <Field label="Name on the ID" value={legalName} onChangeText={setLegalName} autoCapitalize="words" />
      <Field
        label="Date of birth"
        value={dateOfBirth}
        onChangeText={setDateOfBirth}
        autoCapitalize="none"
        placeholder="1990-04-12"
      />
      <Field
        label="Document number"
        value={documentNumber}
        onChangeText={setDocumentNumber}
        autoCapitalize="characters"
      />
      <View style={styles.photoRow}>
        {Platform.OS !== "web" ? <Button title="Take photo" tone="plain" onPress={() => pick(true)} /> : null}
        <Button title="Choose photo" tone="plain" onPress={() => pick(false)} />
      </View>
      <Text style={styles.photo}>{photo ? "Photo added" : "No photo yet"}</Text>
      <ErrorText>{error}</ErrorText>
      <Button title={busy ? "Saving" : "Submit ID"} onPress={submit} disabled={busy} />
      <Button title="Sign out" tone="plain" onPress={signOut} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  choiceLabel: { marginTop: 28, color: colors.secondary, fontSize: 13 },
  photoRow: { marginTop: 8 },
  photo: { marginTop: 8, color: colors.text, fontSize: 16 },
});
