import { NavigationContainer } from "@react-navigation/native";
import { createBottomTabNavigator } from "@react-navigation/bottom-tabs";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { StatusBar } from "expo-status-bar";
import { ReactNode, useCallback, useEffect, useState } from "react";
import { StyleSheet, View } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { api } from "./src/api";
import { AuthProvider, useAuth } from "./src/auth";
import { LiveProvider, useLive } from "./src/live";
import { BackgroundNoteProvider, LocationBridge } from "./src/location/bridge";
import { Loading } from "./src/components/ui";
import { LoginScreen, RegisterScreen, WelcomeScreen } from "./src/screens/AuthScreens";
import { CheckPrompt } from "./src/screens/CheckPrompt";
import { CircleScreen } from "./src/screens/CircleScreen";
import { IdScreen } from "./src/screens/IdScreen";
import { PersonScreen } from "./src/screens/PersonScreen";
import { WatchScreen } from "./src/screens/WatchScreen";
import { colors } from "./src/theme";
import type { RootStackParamList, SafetyCheck, TabParamList } from "./src/types";

const Stack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<TabParamList>();

function MainTabs() {
  return (
    <Tab.Navigator
      screenOptions={{
        headerShown: false,
        tabBarStyle: {
          backgroundColor: colors.bg,
          borderTopColor: colors.line,
          borderTopWidth: 1,
          elevation: 0,
          shadowOpacity: 0,
        },
        tabBarActiveTintColor: colors.text,
        tabBarInactiveTintColor: colors.secondary,
        tabBarLabelStyle: { fontSize: 13, fontWeight: "500" },
        tabBarIcon: () => null,
        tabBarIconStyle: { width: 0, height: 0 },
      }}
    >
      <Tab.Screen name="Circle" component={CircleScreen} options={{ title: "My circle" }} />
      <Tab.Screen name="Watch" component={WatchScreen} options={{ title: "People I watch" }} />
    </Tab.Navigator>
  );
}

function Gate({ children }: { children: ReactNode }) {
  const { token, me } = useAuth();
  const [check, setCheck] = useState<SafetyCheck | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    try {
      const result = await api.pendingCheck(token);
      setCheck(result.check);
    } catch {
      // Keep the last prompt if the network blips.
    }
  }, [token]);

  useEffect(() => {
    load();
    const timer = setInterval(load, 5000);
    return () => clearInterval(timer);
  }, [load]);

  useLive(
    useCallback(
      (event) => {
        if (event.type !== "check" || !event.check || !me || event.check.personId !== me.id) return;
        setCheck(event.check.status === "pending" ? event.check : null);
      },
      [me],
    ),
  );

  return (
    <View style={styles.fill}>
      {children}
      {check ? <CheckPrompt check={check} onDone={() => { setCheck(null); load(); }} /> : null}
    </View>
  );
}

function Root() {
  const { ready, token, me } = useAuth();
  const [backgroundLimited, setBackgroundLimited] = useState(false);
  const onBackground = useCallback((limited: boolean) => setBackgroundLimited(limited), []);

  if (!ready || (token && !me)) {
    return (
      <View style={styles.fill}>
        <Loading />
      </View>
    );
  }

  const signedIn = Boolean(token && me);
  return (
    <BackgroundNoteProvider value={backgroundLimited}>
      <View style={styles.frame}>
        <View style={styles.column}>
          {signedIn && me?.idOnFile ? <LocationBridge onBackground={onBackground} /> : null}
          {signedIn && me?.idOnFile && token ? (
            <LiveProvider token={token}>
              <Gate>
                <Navigator meOnFile />
              </Gate>
            </LiveProvider>
          ) : (
            <Navigator meOnFile={Boolean(me?.idOnFile)} />
          )}
        </View>
      </View>
    </BackgroundNoteProvider>
  );
}

function Navigator({ meOnFile }: { meOnFile: boolean }) {
  const { token } = useAuth();
  return (
    <NavigationContainer>
      <Stack.Navigator screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg } }}>
        {!token ? (
          <>
            <Stack.Screen name="Welcome" component={WelcomeScreen} />
            <Stack.Screen name="Register" component={RegisterScreen} />
            <Stack.Screen name="Login" component={LoginScreen} />
          </>
        ) : !meOnFile ? (
          <Stack.Screen name="Id" component={IdScreen} />
        ) : (
          <>
            <Stack.Screen name="Main" component={MainTabs} />
            <Stack.Screen name="Person" component={PersonScreen} />
          </>
        )}
      </Stack.Navigator>
    </NavigationContainer>
  );
}

export default function App() {
  return (
    <SafeAreaProvider>
      <StatusBar style="dark" />
      <AuthProvider>
        <Root />
      </AuthProvider>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1, backgroundColor: colors.bg },
  frame: { flex: 1, backgroundColor: colors.bg, alignItems: "center" },
  column: { flex: 1, width: "100%", maxWidth: 480 },
});
