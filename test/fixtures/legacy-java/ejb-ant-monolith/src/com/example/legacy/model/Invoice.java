package com.example.legacy.model;

import java.io.Serializable;
import java.math.BigDecimal;
import java.util.Date;

public class Invoice implements Serializable {
    private static final long serialVersionUID = 1L;

    private Long id;
    private String customer;
    private BigDecimal amount;
    private Date issuedAt;

    public Invoice() {}

    public Invoice(Long id, String customer, BigDecimal amount, Date issuedAt) {
        this.id = id;
        this.customer = customer;
        this.amount = amount;
        this.issuedAt = issuedAt;
    }

    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }

    public String getCustomer() { return customer; }
    public void setCustomer(String customer) { this.customer = customer; }

    public BigDecimal getAmount() { return amount; }
    public void setAmount(BigDecimal amount) { this.amount = amount; }

    public Date getIssuedAt() { return issuedAt; }
    public void setIssuedAt(Date issuedAt) { this.issuedAt = issuedAt; }
}
