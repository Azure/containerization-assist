package com.example;

import com.sun.net.httpserver.HttpServer;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpExchange;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;

/**
 * Simple Java HTTP server for build testing.
 * Demonstrates a minimal self-contained application.
 */
public class App {
    private static final int PORT = 8080;

    public static void main(String[] args) throws IOException {
        // Health check mode
        if (args.length > 0 && "health".equals(args[0])) {
            System.out.println("Health check: OK");
            System.exit(0);
            return;
        }

        HttpServer server = HttpServer.create(new InetSocketAddress(PORT), 0);
        
        server.createContext("/", exchange -> {
            String response = "Hello from Java!";
            exchange.sendResponseHeaders(200, response.length());
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes());
            }
        });
        
        server.createContext("/health", exchange -> {
            String response = "OK";
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
