import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

const KEY = "circle_token";

export async function getToken(): Promise<string | null> {
  if (Platform.OS === "web") {
    if (typeof localStorage === "undefined") return null;
    return localStorage.getItem(KEY);
  }
  return SecureStore.getItemAsync(KEY);
}

export async function setToken(token: string): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.setItem(KEY, token);
    return;
  }
  await SecureStore.setItemAsync(KEY, token);
}

export async function clearToken(): Promise<void> {
  if (Platform.OS === "web") {
    localStorage.removeItem(KEY);
    return;
  }
  await SecureStore.deleteItemAsync(KEY);
}
