package com.example.legacy.ejb;

import com.example.legacy.model.Invoice;

import javax.annotation.Resource;
import javax.ejb.Stateless;
import javax.sql.DataSource;
import java.math.BigDecimal;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;

@Stateless
public class InvoiceBean {

    @Resource(lookup = "java:jboss/datasources/InvoicesDS")
    private DataSource dataSource;

    public List<Invoice> listInvoices() {
        List<Invoice> result = new ArrayList<Invoice>();
        String sql = "SELECT id, customer, amount, issued_at FROM invoices ORDER BY id";
        try (Connection c = dataSource.getConnection();
             PreparedStatement ps = c.prepareStatement(sql);
             ResultSet rs = ps.executeQuery()) {
            while (rs.next()) {
                result.add(new Invoice(
                        rs.getLong("id"),
                        rs.getString("customer"),
                        rs.getBigDecimal("amount"),
                        rs.getTimestamp("issued_at")));
            }
        } catch (SQLException e) {
            throw new RuntimeException("Failed to load invoices", e);
        }
        return result;
    }

    public Invoice create(String customer, BigDecimal amount) {
        String sql = "INSERT INTO invoices (customer, amount, issued_at) VALUES (?, ?, NOW())";
        try (Connection c = dataSource.getConnection();
             PreparedStatement ps = c.prepareStatement(sql,
                     PreparedStatement.RETURN_GENERATED_KEYS)) {
            ps.setString(1, customer);
            ps.setBigDecimal(2, amount);
            ps.executeUpdate();
            try (ResultSet keys = ps.getGeneratedKeys()) {
                Long id = keys.next() ? keys.getLong(1) : null;
                return new Invoice(id, customer, amount, new java.util.Date());
            }
        } catch (SQLException e) {
            throw new RuntimeException("Failed to create invoice", e);
        }
    }
}
