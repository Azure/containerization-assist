package com.example;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.InetSocketAddress;
import java.net.URL;

/**
 * Simple Java HTTP server for E2E workflow testing.
 * Self-contained with no external dependencies.
 */
public class App {
    private static final int PORT = 8080;

    public static void main(String[] args) throws IOException {
        // Health check mode - actually hit the health endpoint
        if (args.length > 0 && "health".equals(args[0])) {
            try {
                URL url = new URL("http://localhost:" + PORT + "/health");
                HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                conn.setRequestMethod("GET");
                conn.setConnectTimeout(2000);
                conn.setReadTimeout(2000);
                int responseCode = conn.getResponseCode();
                if (responseCode == 200) {
                    System.out.println("Health check: OK");
                    System.exit(0);
                } else {
                    System.out.println("Health check: FAILED (status " + responseCode + ")");
                    System.exit(1);
                }
            } catch (Exception e) {
                System.out.println("Health check: FAILED (" + e.getMessage() + ")");
                System.exit(1);
            }
            return;
        }

        HttpServer server = HttpServer.create(new InetSocketAddress(PORT), 0);
        
        // Root endpoint
        server.createContext("/", exchange -> {
            String response = "{\"message\": \"Hello from Java!\", \"version\": \"1.0.0\"}";
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, response.length());
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes());
            }
        });
        
        // Health endpoint
        server.createContext("/health", exchange -> {
            String response = "{\"status\": \"healthy\"}";
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, response.length());
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes());
            }
        });

        // Ready endpoint
        server.createContext("/ready", exchange -> {
            String response = "{\"ready\": true}";
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, response.length());
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes());
            }
        });

        server.setExecutor(null);
        server.start();
        System.out.println("Java server running on port " + PORT);
    }
}
