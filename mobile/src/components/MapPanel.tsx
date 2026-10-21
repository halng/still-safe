import { WebView } from "react-native-webview";
import { StyleSheet } from "react-native";

export function MapPanel({ html }: { html: string }) {
  return (
    <WebView
      originWhitelist={["*"]}
      source={{ html }}
      style={styles.map}
      scrollEnabled={false}
    />
  );
}

const styles = StyleSheet.create({
  map: { flex: 1, backgroundColor: "#FFFFFF" },
});
