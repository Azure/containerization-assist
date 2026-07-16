package com.example.legacy.api;

import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/orders")
public class OrderController {

    @GetMapping
    public List<Map<String, Object>> list() {
        return List.of(
                Map.of("id", 1, "customer", "alice", "createdAt", Instant.now().toString()),
                Map.of("id", 2, "customer", "bob", "createdAt", Instant.now().toString()));
    }

    @GetMapping("/{id}")
    public Map<String, Object> get(@PathVariable long id) {
        return Map.of("id", id, "customer", "demo", "createdAt", Instant.now().toString());
    }
}
