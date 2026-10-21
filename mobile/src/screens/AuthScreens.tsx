import { useState } from "react";
import { NativeStackScreenProps } from "@react-navigation/native-stack";
import { ApiError } from "../api";
import { api } from "../api";
import { useAuth } from "../auth";
import { Body, Button, ErrorText, Field, Screen, Title } from "../components/ui";
import type { RootStackParamList } from "../types";

export function WelcomeScreen({ navigation }: NativeStackScreenProps<RootStackParamList, "Welcome">) {
  return (
    <Screen>
      <Title>Circle</Title>
      <Body>Share your precise location with people you trust. They can ask if you are safe.</Body>
      <Body>For people over 16.</Body>
      <Button title="Create account" onPress={() => navigation.navigate("Register")} />
      <Button title="Sign in" tone="plain" onPress={() => navigation.navigate("Login")} />
    </Screen>
  );
}

export function RegisterScreen({ navigation }: NativeStackScreenProps<RootStackParamList, "Register">) {
  const { signIn } = useAuth();
  const [name, setName] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [again, setAgain] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (passphrase !== again) {
      setError("Those passphrases do not match.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const result = await api.register(name.trim(), passphrase);
      await signIn(result.token);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the account.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen scroll>
      <Title>Create account</Title>
      <Body>You will add a government ID next. Circle uses the date of birth on it.</Body>
      <Field label="Name" value={name} onChangeText={setName} autoCapitalize="words" />
      <Field label="Passphrase" value={passphrase} onChangeText={setPassphrase} secureTextEntry autoCapitalize="none" />
      <Field label="Passphrase again" value={again} onChangeText={setAgain} secureTextEntry autoCapitalize="none" />
      <ErrorText>{error}</ErrorText>
      <Button title={busy ? "Creating" : "Create account"} onPress={submit} disabled={busy} />
      <Button title="Back" tone="plain" onPress={() => navigation.goBack()} />
    </Screen>
  );
}

export function LoginScreen({ navigation }: NativeStackScreenProps<RootStackParamList, "Login">) {
  const { signIn } = useAuth();
  const [name, setName] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError("");
    try {
      const result = await api.login(name.trim(), passphrase);
      await signIn(result.token);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not sign in.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen scroll>
      <Title>Sign in</Title>
      <Field label="Name" value={name} onChangeText={setName} autoCapitalize="words" />
      <Field label="Passphrase" value={passphrase} onChangeText={setPassphrase} secureTextEntry autoCapitalize="none" />
      <ErrorText>{error}</ErrorText>
      <Button title={busy ? "Signing in" : "Sign in"} onPress={submit} disabled={busy} />
      <Button title="Back" tone="plain" onPress={() => navigation.goBack()} />
    </Screen>
  );
}
